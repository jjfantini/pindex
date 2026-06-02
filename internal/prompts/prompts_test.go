package prompts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPromptsEmbedInputsAndSchema(t *testing.T) {
	if !strings.Contains(GenerateTOCInit("PAGETEXT"), "PAGETEXT") {
		t.Error("init prompt should embed the page text")
	}
	if !strings.Contains(GenerateTOCInit(""), "physical_index") {
		t.Error("init prompt should ask for physical_index")
	}
	cont := GenerateTOCContinue("PREVJSON", "PARTTEXT")
	if !strings.Contains(cont, "PREVJSON") || !strings.Contains(cont, "PARTTEXT") {
		t.Error("continue prompt should embed previous structure and current part")
	}
	if !strings.Contains(CheckTitleAppearanceInStart("T", "P"), "start_begin") {
		t.Error("appear-in-start prompt should request start_begin")
	}
	if !strings.Contains(CheckTitleAppearance("T", "P"), `"answer"`) {
		t.Error("appearance prompt should request answer")
	}
	if !strings.Contains(TOCDetector("X"), "toc_detected") {
		t.Error("detector prompt should request toc_detected")
	}
	if !strings.Contains(NodeSummary("BODY"), "BODY") {
		t.Error("summary prompt should embed text")
	}
	if !strings.Contains(DocDescription("STRUCT"), "STRUCT") {
		t.Error("description prompt should embed structure")
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
