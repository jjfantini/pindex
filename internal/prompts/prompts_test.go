package prompts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPromptsEmbedInputsAndSchema(t *testing.T) {
	if !strings.Contains(GenerateTOCInit("PAGETEXT").User, "PAGETEXT") {
		t.Error("init prompt should embed the page text in User")
	}
	if !strings.Contains(GenerateTOCInit("").System, "physical_index") {
		t.Error("init prompt System should ask for physical_index")
	}
	cont := GenerateTOCContinue("PREVJSON", "PARTTEXT")
	if !strings.Contains(cont.User, "PREVJSON") || !strings.Contains(cont.User, "PARTTEXT") {
		t.Error("continue prompt should embed previous structure and current part in User")
	}
	if !strings.Contains(CheckTitleAppearanceInStart("T", "P").User, "start_begin") {
		t.Error("appear-in-start prompt should request start_begin")
	}
	if !strings.Contains(CheckTitleAppearance("T", "P").User, `"answer"`) {
		t.Error("appearance prompt should request answer")
	}
	if !strings.Contains(TOCDetector("X").User, "toc_detected") {
		t.Error("detector prompt should request toc_detected")
	}
	if !strings.Contains(NodeSummary("BODY").User, "BODY") {
		t.Error("summary prompt should embed text in User")
	}
	if !strings.Contains(DocDescription("STRUCT").User, "STRUCT") {
		t.Error("description prompt should embed structure in User")
	}
}

// TestPromptSplitKeepsDataOutOfSystem locks the cache-friendliness contract: the
// stable instruction text lives in System and the per-request data lives in User,
// so System is a constant prefix a provider can cache. Every prompt must put its
// interpolated inputs in User and keep them out of System.
func TestPromptSplitKeepsDataOutOfSystem(t *testing.T) {
	const (
		structure = "STRUCTURE_SENTINEL"
		question  = "QUESTION_SENTINEL"
		tried     = "TRIEDPAGES_SENTINEL"
		pages     = "PAGESJSON_SENTINEL"
		text      = "TEXT_SENTINEL"
	)
	cases := []struct {
		name string
		p    Prompt
		data []string // must appear in User, must NOT appear in System
	}{
		{"AskSelectPages", AskSelectPages(structure, question), []string{structure, question}},
		{"AskSelectMore", AskSelectMore(structure, question, tried), []string{structure, question, tried}},
		{"AskAnswer", AskAnswer(question, pages), []string{question, pages}},
		{"AskVerify", AskVerify(question, "ANSWER_SENTINEL", pages), []string{question, "ANSWER_SENTINEL", pages}},
		{"AskAgent", AskAgent(question, structure, "DOCMETA_SENTINEL"), []string{question, structure, "DOCMETA_SENTINEL"}},
		{"GenerateTOCInit", GenerateTOCInit(text), []string{text}},
		{"GenerateTOCContinue", GenerateTOCContinue("PREV_SENTINEL", text), []string{"PREV_SENTINEL", text}},
		{"TOCDetector", TOCDetector(text), []string{text}},
		{"NodeSummary", NodeSummary(text), []string{text}},
		{"JudgeEquivalence", JudgeEquivalence(question, "GOLD_SENTINEL", pages), []string{question, "GOLD_SENTINEL", pages}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.TrimSpace(tc.p.System) == "" {
				t.Fatal("System must be non-empty (the cacheable instruction prefix)")
			}
			for _, d := range tc.data {
				if strings.Contains(tc.p.System, d) {
					t.Errorf("System must not contain request data %q (it would break the cache prefix)", d)
				}
				if !strings.Contains(tc.p.User, d) {
					t.Errorf("User must contain request data %q", d)
				}
			}
		})
	}
}

// TestAskSelectMoreOrderingFix pins the specific regression: the tried-pages list
// used to lead the whole prompt; it must now sit in User, after the stable System
// instructions, so System never varies with which pages were already tried.
func TestAskSelectMoreOrderingFix(t *testing.T) {
	a := AskSelectMore("STRUCT", "Q", "8-10")
	b := AskSelectMore("STRUCT", "Q", "42-44")
	if a.System != b.System {
		t.Error("AskSelectMore System must be identical regardless of tried pages")
	}
	if strings.Contains(a.System, "8-10") {
		t.Error("tried pages must not appear in System")
	}
	if !strings.Contains(a.User, "8-10") {
		t.Error("tried pages must appear in User")
	}
}

func TestSchemasUnmarshal(t *testing.T) {
	var items []TOCItem
	if err := json.Unmarshal([]byte(`[{"structure":"1.1","title":"X","physical_index":"<physical_index_3>"}]`), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].PhysicalIndex != "<physical_index_3>" || items[0].Structure != "1.1" {
		t.Errorf("TOCItem = %+v", items)
	}

	var sb StartBegin
	if err := json.Unmarshal([]byte(`{"thinking":"t","start_begin":"yes"}`), &sb); err != nil {
		t.Fatal(err)
	}
	if sb.StartBegin != "yes" {
		t.Errorf("StartBegin = %+v", sb)
	}

	var ap Appearance
	if err := json.Unmarshal([]byte(`{"answer":"no"}`), &ap); err != nil {
		t.Fatal(err)
	}
	if ap.Answer != "no" {
		t.Errorf("Appearance = %+v", ap)
	}

	var td TOCDetected
	if err := json.Unmarshal([]byte(`{"toc_detected":"yes"}`), &td); err != nil {
		t.Fatal(err)
	}
	if td.TOCDetected != "yes" {
		t.Errorf("TOCDetected = %+v", td)
	}

	var v Verification
	if err := json.Unmarshal([]byte(`{"thinking":"t","verdict":"unsupported","missing":"the $9,999 figure"}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.Verdict != "unsupported" || v.Missing != "the $9,999 figure" {
		t.Errorf("Verification = %+v", v)
	}

	var aa AgentAction
	if err := json.Unmarshal([]byte(`{"thinking":"t","action":"get_pages","pages":"5-7,12"}`), &aa); err != nil {
		t.Fatal(err)
	}
	if aa.Action != "get_pages" || aa.Pages != "5-7,12" {
		t.Errorf("AgentAction = %+v", aa)
	}
	if err := json.Unmarshal([]byte(`{"thinking":"t","action":"answer","answer":"42","pages_used":"5,7"}`), &aa); err != nil {
		t.Fatal(err)
	}
	if aa.Action != "answer" || aa.Answer != "42" || aa.PagesUsed != "5,7" {
		t.Errorf("AgentAction = %+v", aa)
	}
}

// TestAskAgentContract pins the agentic-loop prompt: both actions and the
// reply-JSON shape live in System (the deliberate deviation — a multi-turn loop
// needs the shape to govern every turn), the answer must be grounded only in
// fetched pages, and honesty on a missing answer is required.
func TestAskAgentContract(t *testing.T) {
	p := AskAgent("Q", "STRUCT", "META")
	for _, want := range []string{`"get_pages"`, `"action"`, `"pages"`, `"answer"`, `"pages_used"`, `"thinking"`, "ONLY in page text you fetched", "not found"} {
		if !strings.Contains(p.System, want) {
			t.Errorf("AskAgent System should contain %q", want)
		}
	}
	// Multi-turn: the same System must hold for every question/structure.
	if AskAgent("Q2", "S2", "M2").System != p.System {
		t.Error("AskAgent System must be identical across requests (cacheable prefix)")
	}
}

// TestAskVerifyGrounding pins the fact-checking contract of the high-effort
// verification prompt: claims must be checked against the page text, refusals
// count as supported, and the reply asks for a supported/unsupported verdict.
func TestAskVerifyGrounding(t *testing.T) {
	p := AskVerify("Q", "A", "PAGES")
	sys := strings.ToLower(p.System)
	for _, want := range []string{"fact-checker", "directly supported", "abstention"} {
		if !strings.Contains(sys, want) {
			t.Errorf("AskVerify System should mention %q", want)
		}
	}
	for _, want := range []string{`"verdict"`, `"missing"`, `"thinking"`, "supported", "unsupported"} {
		if !strings.Contains(p.User, want) {
			t.Errorf("AskVerify User should contain %q (reply JSON shape)", want)
		}
	}
}
