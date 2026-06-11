package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	ptree "github.com/jjfantini/pindex/internal/tree"
)

func TestPlainModeStepIsLineOrientedAndControlCodeFree(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, Plain())
	if u.Animated() {
		t.Fatal("plain UI must not animate")
	}
	s := u.Step("extracting pages")
	s.Update("structure group 1/2")
	s.Update("structure group 2/2")
	s.Done("extracted 2 pages")
	out := buf.String()

	for _, want := range []string{
		"• extracting pages…",
		"  › structure group 1/2",
		"  › structure group 2/2",
		"✓ extracted 2 pages (",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plain output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") || strings.Contains(out, "\r") {
		t.Errorf("plain output must not contain ANSI/control codes:\n%q", out)
	}
}

func TestPlainModeStepFail(t *testing.T) {
	var buf bytes.Buffer
	s := New(&buf, Plain()).Step("indexing doc.pdf")
	s.Fail("")
	if !strings.Contains(buf.String(), "✗ indexing doc.pdf (") {
		t.Errorf("Fail with empty message should reuse the label:\n%s", buf.String())
	}
}

func TestStepFinishIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	s := New(&buf, Plain()).Step("work")
	s.Done("done")
	before := buf.String()
	s.Done("again")
	s.Fail("nope")
	s.Update("late")
	if buf.String() != before {
		t.Errorf("calls after finish must be no-ops:\n%q vs %q", before, buf.String())
	}
}

func TestAnimatedStepSpinsAndClears(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, ForceAnimate())
	if !u.Animated() {
		t.Fatal("ForceAnimate must animate")
	}
	s := u.Step("working")
	time.Sleep(3 * spinnerInterval)
	s.Done("finished")
	out := buf.String()
	if !strings.Contains(out, "\r") || !strings.Contains(out, "\x1b[2K") {
		t.Errorf("animated mode should redraw in place:\n%q", out)
	}
	frameSeen := false
	for _, f := range spinnerFrames {
		if strings.Contains(out, f) {
			frameSeen = true
			break
		}
	}
	if !frameSeen {
		t.Errorf("animated mode should render spinner frames:\n%q", out)
	}
	// Color escapes sit between the message and the duration, so match parts.
	if !strings.Contains(out, "finished") || !strings.HasSuffix(out, ")\x1b[0m\n") {
		t.Errorf("final line missing:\n%q", out)
	}
}

func TestNoAnimationKeepsLineOutput(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, ForceAnimate(), NoAnimation())
	if u.Animated() {
		t.Fatal("NoAnimation must win over ForceAnimate")
	}
	s := u.Step("working")
	s.Done("")
	if strings.Contains(buf.String(), "\r") {
		t.Errorf("NoAnimation output must be line-oriented:\n%q", buf.String())
	}
}

func TestStatusHelpers(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, Plain())
	u.Successf("ok %d", 1)
	u.Warnf("careful")
	u.Errorf("broken")
	u.Infof("note")
	u.Notef("wrote to %s", "x/y")
	out := buf.String()
	for _, want := range []string{"✓ ok 1", "⚠ careful", "✗ broken", "• note", "› wrote to x/y"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHeader(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, Plain()).Header("index", "sample.pdf")
	out := buf.String()
	for _, want := range []string{"pindex", "index", "· sample.pdf"} {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q: %q", want, out)
		}
	}
}

func TestSummaryBox(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, Plain())
	box := u.SummaryBox("index complete", [][2]string{{"doc id", "abc123"}, {"pages", "94"}})
	for _, want := range []string{"index complete", "doc id", "abc123", "pages", "94", "╭", "╰"} {
		if !strings.Contains(box, want) {
			t.Errorf("box missing %q:\n%s", want, box)
		}
	}
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, Plain())
	tbl := u.Table([]string{"stage", "rate"}, [][]string{{"extraction", "83.3%"}, {"answer", "66.7%"}})
	for _, want := range []string{"stage", "rate", "extraction", "83.3%", "answer", "╭"} {
		if !strings.Contains(tbl, want) {
			t.Errorf("table missing %q:\n%s", want, tbl)
		}
	}
}

func TestDocTreeTruncates(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, Plain())
	nodes := []ptree.TreeNode{
		{Title: "Intro", StartIndex: 1, EndIndex: 2, Nodes: []ptree.TreeNode{
			{Title: "Background", StartIndex: 2, EndIndex: 2},
		}},
		{Title: "Methods", StartIndex: 3, EndIndex: 4},
	}
	out := u.DocTree("doc.pdf", nodes, 2)
	for _, want := range []string{"doc.pdf", "Intro", "p.1–2", "Background", "p.2", "+1 more sections"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Methods") {
		t.Errorf("tree should have truncated past the node budget:\n%s", out)
	}
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	u := New(&buf, Plain())
	l := u.NewLogger(false)
	l.Debug("hidden detail")
	l.Info("visible info")
	out := buf.String()
	if strings.Contains(out, "hidden detail") {
		t.Errorf("debug must be suppressed without verbose:\n%s", out)
	}
	if !strings.Contains(out, "visible info") {
		t.Errorf("info must show:\n%s", out)
	}

	buf.Reset()
	u.NewLogger(true).Debug("now visible", "key", "val")
	if !strings.Contains(buf.String(), "now visible") || !strings.Contains(buf.String(), "key") {
		t.Errorf("verbose logger must show debug with fields:\n%s", buf.String())
	}
}

func TestPindexPlainEnvForcesPlain(t *testing.T) {
	t.Setenv("PINDEX_PLAIN", "1")
	var buf bytes.Buffer
	u := New(&buf)
	if u.Animated() {
		t.Fatal("PINDEX_PLAIN must disable animation")
	}
	u.Successf("done")
	if strings.Contains(buf.String(), "\x1b") {
		t.Errorf("PINDEX_PLAIN output must be uncolored:\n%q", buf.String())
	}
}

func TestFmtDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "500ms"},
		{1230 * time.Millisecond, "1.2s"},
		{75 * time.Second, "1m15s"},
	}
	for _, c := range cases {
		if got := fmtDur(c.d); got != c.want {
			t.Errorf("fmtDur(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
