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
}
