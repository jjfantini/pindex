// Command adjudicate serves a local web UI for human adjudication of
// FinanceBench misses: every non-AL answer record across all docs and efforts,
// with the question, gold answer, gold-page text, pindex's full answer,
// reasoning, and cited-page text side by side. Labels (MVA/BE/SEDC/NAL) and a
// label reason set in the UI are written back to the per-question answer
// records — the benchmark's source of truth — and the aggregator is re-run, so
// "Apply Now" is the whole relabel-and-rescore loop.
//
// Run from the repo root:
//
//	go run ./eval/financebench/adjudicate
//
// Flags: -results, -ws (page-text workspace), -addr.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jjfantini/pindex/internal/exportout"
	"github.com/jjfantini/pindex/internal/store"
)

//go:embed index.html
var ui embed.FS

//go:embed findings.json
var findingsJSON []byte

// finding is a pre-researched judge assessment for one question — Claude's
// PDF-grounded verdict, reasoning, and evidence quotes, shown in the UI next
// to the human editor. Suggestions only: the human still sets the label.
type finding struct {
	Suggested       string   `json:"suggested"`
	Verdict         string   `json:"verdict"`
	Reasoning       string   `json:"reasoning"`
	Evidence        []string `json:"evidence,omitempty"`
	SuggestedReason string   `json:"suggested_reason,omitempty"`
}

func loadFindings() (map[string]finding, error) {
	out := map[string]finding{}
	if err := json.Unmarshal(findingsJSON, &out); err != nil {
		return nil, fmt.Errorf("findings.json: %w", err)
	}
	return out, nil
}

var efforts = []string{"low", "medium", "high", "ultra"}

// labelNames are the human adjudication labels; AL is deliberately absent —
// only the judge assigns AL, a human never grades an answer up to it.
var labelNames = map[string]string{
	"MVA":  "Multiple Valid Approaches",
	"BE":   "Benchmark Error",
	"SEDC": "Subjective Evaluation, Different Conclusions",
	"NAL":  "Not Aligned",
}

// recordRef ties a loaded answer record to the file it came from.
type recordRef struct {
	Path   string
	Effort string
	Rec    exportout.AnswerRecord
}

type apiPage struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

type apiRecord struct {
	Effort        string `json:"effort"`
	Predicted     string `json:"predicted"`
	Reasoning     string `json:"reasoning,omitempty"`
	Verification  string `json:"verification,omitempty"`
	SelectedPages string `json:"selected_pages,omitempty"`
	CitedPages    []int  `json:"cited_pages,omitempty"`
	Label         string `json:"label"`
	LabelReason   string `json:"label_reason,omitempty"`
	Hallucinated  bool   `json:"hallucinated"`
	RetrievalOK   bool   `json:"retrieval_ok"`
}

type apiQuestion struct {
	ID        string      `json:"id"`
	Doc       string      `json:"doc"`
	Question  string      `json:"question"`
	Gold      string      `json:"gold"`
	GoldPages []int       `json:"gold_pages"` // FinanceBench numbering (0-indexed vs PDF)
	GoldText  []apiPage   `json:"gold_text"`  // text at PDF page = gold_page + 1
	CitedText []apiPage   `json:"cited_text"` // union of cited pages across efforts
	Records   []apiRecord `json:"records"`
	Finding   *finding    `json:"finding,omitempty"`
}

type apiData struct {
	Model     string        `json:"model"`
	WSNote    string        `json:"ws_note,omitempty"`
	Questions []apiQuestion `json:"questions"`
}

type server struct {
	resultsRoot string
	model       string
	byID        map[string][]*recordRef // question id -> its non-AL records
	pages       *pageSource
	findings    map[string]finding
}

// runAggregate rebuilds the derived artifacts and returns the printed
// scoreboard. A var so tests can stub it.
var runAggregate = func(resultsRoot string) (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(resultsRoot)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("go", "run", "./eval/financebench/aggregate", abs)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("aggregate: %w\n%s", err, out)
	}
	return string(out), nil
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s — run from inside the repo", dir)
		}
		dir = parent
	}
}

// pageSource lazily loads per-doc page text from a pindex workspace.
type pageSource struct {
	st    *store.Store
	ids   map[string]string // normalized doc name -> doc id
	cache map[string]map[int]string
	note  string
}

func newPageSource(ws string) *pageSource {
	ps := &pageSource{ids: map[string]string{}, cache: map[string]map[int]string{}}
	st, err := store.Open(ws)
	if err != nil {
		ps.note = fmt.Sprintf("workspace %s unavailable (%v) — page text omitted", ws, err)
		return ps
	}
	rows, err := st.List()
	if err != nil {
		ps.note = fmt.Sprintf("workspace catalog unreadable (%v) — page text omitted", err)
		return ps
	}
	ps.st = st
	for _, r := range rows {
		ps.ids[normalizeName(r.DocName)] = r.ID
	}
	return ps
}

func normalizeName(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".pdf")
}

// text returns the content of physical (1-based) pages of doc, skipping pages
// it cannot resolve.
func (ps *pageSource) text(doc string, pages []int) []apiPage {
	if ps.st == nil {
		return nil
	}
	key := normalizeName(doc)
	byPage, ok := ps.cache[key]
	if !ok {
		id, found := ps.ids[key]
		if !found {
			return nil
		}
		d, err := ps.st.Load(id)
		if err != nil {
			return nil
		}
		byPage = make(map[int]string, len(d.Pages))
		for _, p := range d.Pages {
			byPage[p.Page] = p.Content
		}
		ps.cache[key] = byPage
	}
	var out []apiPage
	for _, p := range pages {
		if t, ok := byPage[p]; ok {
			out = append(out, apiPage{Page: p, Text: t})
		}
	}
	return out
}

// load scans <root>/<model>/<effort>/<DOC>/answers/*.json for non-AL records.
func (s *server) load() error {
	s.byID = map[string][]*recordRef{}
	models, err := os.ReadDir(s.resultsRoot)
	if err != nil {
		return err
	}
	for _, m := range models {
		if !m.IsDir() {
			continue
		}
		s.model = m.Name()
		for _, eff := range efforts {
			paths, _ := filepath.Glob(filepath.Join(s.resultsRoot, m.Name(), eff, "*", "answers", "*.json"))
			for _, p := range paths {
				var rec exportout.AnswerRecord
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				if err := json.Unmarshal(b, &rec); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
				if rec.Label == "AL" || rec.Error != "" {
					continue
				}
				s.byID[rec.FinancebenchID] = append(s.byID[rec.FinancebenchID],
					&recordRef{Path: p, Effort: eff, Rec: rec})
			}
		}
	}
	return nil
}

func (s *server) data() apiData {
	d := apiData{Model: s.model, WSNote: s.pages.note}
	ids := make([]string, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		refs := s.byID[id]
		first := refs[0].Rec
		q := apiQuestion{
			ID:        id,
			Doc:       strings.TrimSuffix(first.DocName, ".pdf"),
			Question:  first.Question,
			Gold:      first.GoldAnswer,
			GoldPages: first.GoldPages,
		}
		// FinanceBench evidence pages are 0-indexed against these PDFs.
		var phys []int
		for _, p := range first.GoldPages {
			phys = append(phys, p+1)
		}
		q.GoldText = s.pages.text(first.DocName, phys)
		cited := map[int]bool{}
		for _, r := range refs {
			q.Records = append(q.Records, apiRecord{
				Effort:        r.Effort,
				Predicted:     r.Rec.Predicted,
				Reasoning:     r.Rec.Reasoning,
				Verification:  r.Rec.Verification,
				SelectedPages: r.Rec.SelectedPages,
				CitedPages:    r.Rec.CitedPages,
				Label:         r.Rec.Label,
				LabelReason:   r.Rec.LabelReason,
				Hallucinated:  r.Rec.Hallucinated,
				RetrievalOK:   r.Rec.RetrievalOK,
			})
			for _, c := range r.Rec.CitedPages {
				cited[c] = true
			}
		}
		var cs []int
		for c := range cited {
			cs = append(cs, c)
		}
		sort.Ints(cs)
		q.CitedText = s.pages.text(first.DocName, cs)
		if f, ok := s.findings[id]; ok {
			q.Finding = &f
		}
		d.Questions = append(d.Questions, q)
	}
	return d
}

type applyChange struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

type applyRequest struct {
	Changes []applyChange `json:"changes"`
}

type applyResponse struct {
	Updated    []string `json:"updated"`
	Scoreboard string   `json:"scoreboard"`
}

// wrapReason puts a free-text reason into the established label_reason shape,
// passing through reasons that already carry it (or are empty).
func wrapReason(label, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" || strings.HasPrefix(reason, "Label:") {
		return reason
	}
	return fmt.Sprintf("Label: %s - %s\n\nDetailed Reason: %s Adjudicated by maintainer %s.",
		label, labelNames[label], reason, time.Now().Format("2006-01-02"))
}

func (s *server) apply(req applyRequest) (applyResponse, error) {
	var resp applyResponse
	for _, ch := range req.Changes {
		if _, ok := labelNames[ch.Label]; !ok {
			return resp, fmt.Errorf("question %s: label %q not allowed (want MVA|BE|SEDC|NAL — AL is judge-only)", ch.ID, ch.Label)
		}
		refs, ok := s.byID[ch.ID]
		if !ok {
			return resp, fmt.Errorf("unknown question id %q", ch.ID)
		}
		reason := wrapReason(ch.Label, ch.Reason)
		for _, r := range refs {
			r.Rec.Label = ch.Label
			r.Rec.LabelReason = reason
			b, err := json.MarshalIndent(r.Rec, "", "  ")
			if err != nil {
				return resp, err
			}
			if err := os.WriteFile(r.Path, append(b, '\n'), 0o644); err != nil {
				return resp, err
			}
			resp.Updated = append(resp.Updated, fmt.Sprintf("%s @ %s -> %s", ch.ID, r.Effort, ch.Label))
		}
	}
	if len(resp.Updated) > 0 {
		out, err := runAggregate(s.resultsRoot)
		if err != nil {
			return resp, err
		}
		resp.Scoreboard = out
	}
	return resp, nil
}

func main() {
	results := flag.String("results", "eval/financebench/results", "results tree root")
	ws := flag.String("ws", "eval/financebench/testdata/ws", "pindex workspace for page text")
	addr := flag.String("addr", "localhost:8787", "listen address")
	flag.Parse()

	s := &server{resultsRoot: *results, pages: newPageSource(*ws)}
	var err error
	if s.findings, err = loadFindings(); err != nil {
		log.Fatal(err)
	}
	if err := s.load(); err != nil {
		log.Fatal(err)
	}
	if s.pages.note != "" {
		log.Print(s.pages.note)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui)))
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.data())
	})
	mux.HandleFunc("/api/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var req applyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := s.apply(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Reload so the UI reflects what is now on disk.
		if err := s.load(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp)
	})

	log.Printf("adjudication UI: http://%s/ (results=%s)", *addr, *results)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Print(err)
	}
}
