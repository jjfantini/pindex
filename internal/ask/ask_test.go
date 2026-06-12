package ask

import (
	"context"
	"strings"
	"testing"

	"github.com/jjfantini/pindex/internal/llm"
	"github.com/jjfantini/pindex/internal/tree"
)

func sampleDoc() tree.Document {
	return tree.Document{
		Type: tree.DocPDF, PageCount: 3,
		Structure: []tree.TreeNode{
			{Title: "Intro", NodeID: "0000", StartIndex: 1, EndIndex: 1},
			{Title: "Financials", NodeID: "0001", StartIndex: 2, EndIndex: 3},
		},
		Pages: []tree.PageContent{
			{Page: 1, Content: "intro text"},
			{Page: 2, Content: "Revenue was $1,234 in 2023."},
			{Page: 3, Content: "more financials"},
		},
	}
}

func TestAskSelectsThenAnswers(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"thinking":"financials live on p2","pages":"2"}`},
		llm.MockResponse{Content: `{"thinking":"found it","answer":"Revenue was $1,234.","pages_used":"2"}`},
	)
	ans, err := New(mock, "m").Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q", ans.Text)
	}
	if len(ans.CitedPages) != 1 || ans.CitedPages[0] != 2 {
		t.Errorf("cited = %v want [2]", ans.CitedPages)
	}
	if ans.SelectedPages != "2" {
		t.Errorf("selected = %q want 2", ans.SelectedPages)
	}
	if mock.CallCount() != 2 {
		t.Errorf("calls = %d want 2", mock.CallCount())
	}
	// The answer prompt must contain the fetched page-2 content (grounding).
	calls := mock.Calls()
	if !strings.Contains(calls[1].Messages[len(calls[1].Messages)-1].Content, "Revenue was $1,234") {
		t.Error("answer prompt User turn should embed the fetched page content")
	}
}

func TestAskRetriesInvalidPageSelector(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"pages":"garbage"}`}, // invalid selector -> retry
		llm.MockResponse{Content: `{"pages":"2"}`},       // valid
		llm.MockResponse{Content: `{"answer":"ok","pages_used":"2"}`},
	)
	ans, err := New(mock, "m").Ask(context.Background(), sampleDoc(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "ok" {
		t.Errorf("answer = %q", ans.Text)
	}
	if mock.CallCount() != 3 {
		t.Errorf("calls = %d want 3 (one select retry)", mock.CallCount())
	}
}

func TestParseEffort(t *testing.T) {
	for in, want := range map[string]Effort{"": EffortLow, "LOW": EffortLow, "medium": EffortMedium, "high": EffortHigh, "ultra": EffortUltra} {
		if got, err := ParseEffort(in); err != nil || got != want {
			t.Errorf("ParseEffort(%q) = %v,%v want %v", in, got, err, want)
		}
	}
	if _, err := ParseEffort("bogus"); err == nil {
		t.Error("bogus effort should error")
	}
}

// --- high effort: the agentic loop ---

const (
	agentGetP2  = `{"thinking":"financials live on p2","action":"get_pages","pages":"2"}`
	agentAnswer = `{"thinking":"found it on p2","action":"answer","answer":"Revenue was $1,234.","pages_used":"2"}`
)

func TestAskHighAgenticFetchThenAnswer(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2},  // turn 1: fetch
		llm.MockResponse{Content: agentAnswer}, // turn 2: answer
	)
	a := New(mock, "m")
	a.Effort = EffortHigh
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q", ans.Text)
	}
	if ans.SelectedPages != "2" {
		t.Errorf("selected = %q want 2 (union of fetched pages)", ans.SelectedPages)
	}
	if len(ans.CitedPages) != 1 || ans.CitedPages[0] != 2 {
		t.Errorf("cited = %v want [2]", ans.CitedPages)
	}
	if ans.Verification != "" {
		t.Errorf("verification = %q want \"\" (high never verifies)", ans.Verification)
	}
	if ans.Steps != 2 {
		t.Errorf("steps = %d want 2", ans.Steps)
	}
	if mock.CallCount() != 2 {
		t.Fatalf("calls = %d want 2 (get_pages, answer)", mock.CallCount())
	}
	// The second turn must see the fetched page text as a user turn (grounding).
	calls := mock.Calls()
	msgs := calls[1].Messages
	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "Revenue was $1,234 in 2023.") {
		t.Errorf("the answer turn should end with a user message carrying the fetched page text, got %q %q", last.Role, last.Content)
	}
	// The agent's own get_pages action must be in the transcript as an assistant turn.
	if prev := msgs[len(msgs)-2]; prev.Role != llm.RoleAssistant || !strings.Contains(prev.Content, `"get_pages"`) {
		t.Errorf("the transcript should carry the assistant's get_pages action, got %q %q", prev.Role, prev.Content)
	}
}

func TestAskHighAgenticRedirectsAnswerBeforeFetch(t *testing.T) {
	// Grounding is enforced mechanically: an answer before any get_pages is not
	// returned — the agent is redirected (one loop turn) to read pages first.
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentAnswer}, // redirected: nothing fetched yet
		llm.MockResponse{Content: agentGetP2},  // fetch
		llm.MockResponse{Content: agentAnswer}, // grounded answer
	)
	a := New(mock, "m")
	a.Effort = EffortHigh
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q", ans.Text)
	}
	if ans.SelectedPages != "2" {
		t.Errorf("selected = %q want 2 (fetch forced before answering)", ans.SelectedPages)
	}
	if ans.Steps != 3 {
		t.Errorf("steps = %d want 3 (redirect consumes a turn)", ans.Steps)
	}
	if mock.CallCount() != 3 {
		t.Fatalf("calls = %d want 3 (redirected answer, get_pages, answer)", mock.CallCount())
	}
	// The turn after the redirect must carry the redirect message to the model.
	msgs := mock.Calls()[1].Messages
	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "without reading any pages") {
		t.Errorf("the turn after a premature answer should end with the redirect message, got %q %q", last.Role, last.Content)
	}
}

func TestAskHighAgenticCapForcesAnswer(t *testing.T) {
	rs := make([]llm.MockResponse, 0, agentMaxIterations+1)
	for i := 0; i < agentMaxIterations; i++ {
		rs = append(rs, llm.MockResponse{Content: agentGetP2}) // never answers
	}
	rs = append(rs, llm.MockResponse{Content: agentAnswer}) // forced final answer
	mock := llm.NewMock("m", rs...)
	a := New(mock, "m")
	a.Effort = EffortHigh
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q", ans.Text)
	}
	if ans.Steps != agentMaxIterations+1 {
		t.Errorf("steps = %d want %d", ans.Steps, agentMaxIterations+1)
	}
	if mock.CallCount() != agentMaxIterations+1 {
		t.Fatalf("calls = %d want %d (cap + forced answer)", mock.CallCount(), agentMaxIterations+1)
	}
	// The forced turn must carry the budget-exhausted instruction as a user message.
	calls := mock.Calls()
	msgs := calls[len(calls)-1].Messages
	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "Iteration budget exhausted") {
		t.Errorf("forced-answer turn should end with the budget-exhausted user message, got %q %q", last.Role, last.Content)
	}
}

func TestAskHighAgenticNeverAnswersFailsTyped(t *testing.T) {
	mock := llm.NewMock("m")
	mock.Default = llm.MockResponse{Content: agentGetP2} // keeps fetching forever
	a := New(mock, "m")
	a.Effort = EffortHigh
	_, err := a.Ask(context.Background(), sampleDoc(), "q")
	if err == nil {
		t.Fatal("an agent that never answers must surface a typed error (no silent empty)")
	}
	if !strings.Contains(err.Error(), "ask: agent: no answer") {
		t.Errorf("err = %v want the typed no-answer error", err)
	}
	// 8 loop turns + the forced-answer turn burning all its validation attempts.
	if want := agentMaxIterations + 3; mock.CallCount() != want {
		t.Errorf("calls = %d want %d", mock.CallCount(), want)
	}
}

func TestAskHighAgenticInvalidActionRetries(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"thinking":"hm","action":"fly"}`}, // invalid action -> validation retry
		llm.MockResponse{Content: agentGetP2},                         // corrected: fetch
		llm.MockResponse{Content: agentAnswer},                        // answer
	)
	a := New(mock, "m")
	a.Effort = EffortHigh
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q (validation retry should recover)", ans.Text)
	}
	if mock.CallCount() != 3 {
		t.Errorf("calls = %d want 3 (one validation retry, then fetch, then answer)", mock.CallCount())
	}
	// The retry must carry the correction message inside the same CompleteJSON call.
	calls := mock.Calls()
	msgs := calls[1].Messages
	if !strings.Contains(msgs[len(msgs)-1].Content, "not valid for the required JSON schema") {
		t.Error("the retry turn should carry the schema-correction message")
	}
}

// --- ultra effort: agentic loop + verification ---

func TestAskUltraVerificationSupported(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2},
		llm.MockResponse{Content: agentAnswer},
		llm.MockResponse{Content: `{"thinking":"all claims on p2","verdict":"supported","missing":""}`}, // verify
	)
	a := New(mock, "m")
	a.Effort = EffortUltra
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q", ans.Text)
	}
	if ans.Verification != "supported" {
		t.Errorf("verification = %q want supported", ans.Verification)
	}
	if mock.CallCount() != 3 {
		t.Fatalf("calls = %d want 3 (get_pages, answer, verify)", mock.CallCount())
	}
	// The verify prompt must embed the cited page's content AND the answer (grounding).
	calls := mock.Calls()
	user := calls[2].Messages[len(calls[2].Messages)-1].Content
	if !strings.Contains(user, "Revenue was $1,234 in 2023.") {
		t.Error("verify prompt should embed the cited page content")
	}
	if !strings.Contains(user, "Revenue was $1,234.") {
		t.Error("verify prompt should embed the answer under check")
	}
}

func TestAskUltraCorrectiveContinuationRecoversUnsupported(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2}, // turn 1: fetch
		llm.MockResponse{Content: `{"thinking":"t","action":"answer","answer":"Revenue was $9,999.","pages_used":"2"}`}, // turn 2: wrong answer
		llm.MockResponse{Content: `{"verdict":"unsupported","missing":"$9,999 not on the pages"}`},                      // verify 1
		llm.MockResponse{Content: `{"thinking":"recheck p3","action":"get_pages","pages":"3"}`},                         // continuation: fetch more
		llm.MockResponse{Content: agentAnswer},                                                       // continuation: corrected answer
		llm.MockResponse{Content: `{"thinking":"now supported","verdict":"supported","missing":""}`}, // verify 2
	)
	a := New(mock, "m")
	a.Effort = EffortUltra
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q (continuation should replace the unsupported answer)", ans.Text)
	}
	if ans.Verification != "supported" {
		t.Errorf("verification = %q want supported", ans.Verification)
	}
	if ans.SelectedPages != "2,3" {
		t.Errorf("selected = %q want 2,3 (fetch union across the continuation)", ans.SelectedPages)
	}
	if mock.CallCount() != 6 {
		t.Errorf("calls = %d want 6 (fetch, answer, verify, fetch, answer, verify)", mock.CallCount())
	}
	// The continuation must run on the SAME conversation, fed the fact-checker's findings.
	calls := mock.Calls()
	msgs := calls[3].Messages
	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "fact-checker") || !strings.Contains(last.Content, "$9,999 not on the pages") {
		t.Errorf("continuation turn should end with the fact-checker findings, got %q %q", last.Role, last.Content)
	}
}

func TestAskUltraBothVerifiesUnsupportedReturnsOriginal(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2},
		llm.MockResponse{Content: `{"thinking":"t","action":"answer","answer":"Revenue was $9,999.","pages_used":"2"}`},
		llm.MockResponse{Content: `{"verdict":"unsupported","missing":"$9,999 not on the pages"}`},
		llm.MockResponse{Content: `{"thinking":"t","action":"answer","answer":"Revenue was $5,555.","pages_used":"3"}`},
		llm.MockResponse{Content: `{"verdict":"unsupported","missing":"$5,555 not on the pages"}`},
	)
	a := New(mock, "m")
	a.Effort = EffortUltra
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $9,999." {
		t.Errorf("answer = %q (must return the ORIGINAL answer, never an unverified replacement)", ans.Text)
	}
	if ans.Verification != "unsupported" {
		t.Errorf("verification = %q want unsupported", ans.Verification)
	}
	if mock.CallCount() != 5 {
		t.Errorf("calls = %d want 5", mock.CallCount())
	}
}

func TestAskUltraSkipsVerificationOnRefusal(t *testing.T) {
	refusal := `{"thinking":"t","action":"answer","answer":"I cannot find it in the document.","pages_used":""}`
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2},
		llm.MockResponse{Content: refusal}, // redirected once (give-up nudge)
		llm.MockResponse{Content: refusal}, // honest refusal stands
	)
	a := New(mock, "m")
	a.Effort = EffortUltra
	ans, err := a.Ask(context.Background(), sampleDoc(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "cannot find") {
		t.Errorf("answer = %q want the honest refusal", ans.Text)
	}
	if ans.Verification != "" {
		t.Errorf("verification = %q want \"\" (refusals are never verified)", ans.Verification)
	}
	if mock.CallCount() != 3 {
		t.Errorf("calls = %d want 3 (fetch, redirected refusal, final refusal; no verification)", mock.CallCount())
	}
}

func TestAskUltraVerifyTransportErrorFailsLoudly(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2},
		llm.MockResponse{Content: agentAnswer},
		llm.MockResponse{Err: context.DeadlineExceeded}, // verify: transport error
	)
	a := New(mock, "m")
	a.Effort = EffortUltra
	if _, err := a.Ask(context.Background(), sampleDoc(), "q"); err == nil {
		t.Error("a verification transport error must fail the call (no silent failures)")
	}
}

func TestAskMediumFetchMoreRecoversRefusal(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"pages":"1"}`},                                     // select
		llm.MockResponse{Content: `{"answer":"I cannot find it.","pages_used":"1"}`},   // answer: refusal
		llm.MockResponse{Content: `{"pages":"2"}`},                                     // select-more
		llm.MockResponse{Content: `{"answer":"Revenue was $1,234.","pages_used":"2"}`}, // answer: success
	)
	a := New(mock, "m")
	a.Effort = EffortMedium
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q (medium should recover via fetch-more)", ans.Text)
	}
	if ans.SelectedPages != "1,2" {
		t.Errorf("selected = %q want 1,2", ans.SelectedPages)
	}
	if mock.CallCount() != 4 {
		t.Errorf("calls = %d want 4 (select, answer-refusal, select-more, answer)", mock.CallCount())
	}
	if ans.Verification != "" {
		t.Errorf("verification = %q want \"\" (medium never verifies)", ans.Verification)
	}
}

func TestAskLowDoesNotFetchMore(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"pages":"1"}`},
		llm.MockResponse{Content: `{"answer":"I cannot find it.","pages_used":"1"}`},
	)
	ans, err := New(mock, "m").Ask(context.Background(), sampleDoc(), "q") // low default
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "cannot find") {
		t.Errorf("low effort should return the honest refusal as-is, got %q", ans.Text)
	}
	if mock.CallCount() != 2 {
		t.Errorf("calls = %d want 2 (no fetch-more at low)", mock.CallCount())
	}
	if ans.Verification != "" {
		t.Errorf("verification = %q want \"\" (low never verifies)", ans.Verification)
	}
}

// oversizedDoc renders to well over the unknown-model structure budget (the
// PEPSICO_2022_10K failure shape: hundreds of nodes with multi-paragraph
// summaries that overflow the model context in one prompt).
func oversizedDoc() tree.Document {
	nodes := make([]tree.TreeNode, 200)
	for i := range nodes {
		nodes[i] = tree.TreeNode{
			Title: "Section", StartIndex: i + 1, EndIndex: i + 1,
			Summary: strings.Repeat("s", 2_500),
		}
	}
	return tree.Document{
		Type: tree.DocPDF, PageCount: 200,
		Structure: nodes,
		Pages:     []tree.PageContent{{Page: 2, Content: "Revenue was $1,234 in 2023."}},
	}
}

// Regression: a structure bigger than the prompt budget must be degraded, not
// embedded wholesale (PEPSICO_2022_10K: every ask died with "prompt is too
// long: 205330 tokens > 200000 maximum" at select-pages).
func TestAskBudgetsOversizedStructure(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"thinking":"p2","pages":"2"}`},
		llm.MockResponse{Content: `{"thinking":"found","answer":"ok","pages_used":"2"}`},
	)
	if _, err := New(mock, "m").Ask(context.Background(), oversizedDoc(), "What was revenue?"); err != nil {
		t.Fatal(err)
	}
	sel := mock.Calls()[0]
	user := sel.Messages[len(sel.Messages)-1].Content
	if len(user) > llm.StructureBudget("m")+2_000 { // small allowance for instructions
		t.Errorf("select-pages prompt = %d chars, structure was not budgeted", len(user))
	}
}

func TestAskAgenticBudgetsOversizedStructure(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"action":"get_pages","pages":"2"}`},
		llm.MockResponse{Content: `{"action":"answer","answer":"ok","pages_used":"2"}`},
	)
	a := New(mock, "m")
	a.Effort = EffortHigh
	if _, err := a.Ask(context.Background(), oversizedDoc(), "What was revenue?"); err != nil {
		t.Fatal(err)
	}
	first := mock.Calls()[0]
	total := 0
	for _, m := range first.Messages {
		total += len(m.Content)
	}
	if total > llm.StructureBudget("m")+5_000 { // system + user instruction allowance
		t.Errorf("agent opening prompt = %d chars, structure was not budgeted", total)
	}
}

// A 1M-context model gets the same oversized structure untouched — the budget
// is model-aware, and degradation is only a safety floor there.
func TestAskBigContextModelGetsFullStructure(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: `{"thinking":"p2","pages":"2"}`},
		llm.MockResponse{Content: `{"thinking":"found","answer":"ok","pages_used":"2"}`},
	)
	if _, err := New(mock, "claude-sonnet-4-6").Ask(context.Background(), oversizedDoc(), "What was revenue?"); err != nil {
		t.Fatal(err)
	}
	user := mock.Calls()[0].Messages[len(mock.Calls()[0].Messages)-1].Content
	if strings.Contains(user, "…") {
		t.Error("structure was truncated despite fitting the 1M-context budget")
	}
	if len(user) < 500_000 {
		t.Errorf("select prompt = %d chars; full oversized structure should be embedded", len(user))
	}
}

const agentRefusal = `{"thinking":"the fetched pages lack it","action":"answer","answer":"The document does not specify this.","pages_used":"2"}`

// Regression (Amex 12(b)): a "not found / not specified" answer with turns
// remaining is redirected ONCE to keep exploring instead of surrendering on
// the first fetched range.
func TestAskAgenticNotFoundRedirectExploresThenAnswers(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2},   // fetch wrong-ish pages
		llm.MockResponse{Content: agentRefusal}, // gives up -> redirected
		llm.MockResponse{Content: `{"thinking":"trying page 1","action":"get_pages","pages":"1"}`},
		llm.MockResponse{Content: agentAnswer}, // finds it after the nudge
	)
	a := New(mock, "m")
	a.Effort = EffortHigh
	ans, err := a.Ask(context.Background(), sampleDoc(), "What was revenue?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Revenue was $1,234." {
		t.Errorf("answer = %q (the post-redirect answer should be returned)", ans.Text)
	}
	if ans.SelectedPages != "1,2" {
		t.Errorf("selected = %q want 1,2 (both fetches)", ans.SelectedPages)
	}
	if mock.CallCount() != 4 {
		t.Fatalf("calls = %d want 4 (fetch, refusal, fetch, answer)", mock.CallCount())
	}
	// The turn after the refusal must carry the redirect message.
	msgs := mock.Calls()[2].Messages
	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "do not conclude the document lacks") {
		t.Errorf("expected the give-up redirect as the next user turn, got %q %q", last.Role, last.Content)
	}
}

func TestAskAgenticNotFoundRedirectFiresOnlyOnce(t *testing.T) {
	mock := llm.NewMock("m",
		llm.MockResponse{Content: agentGetP2},
		llm.MockResponse{Content: agentRefusal}, // redirected
		llm.MockResponse{Content: agentRefusal}, // honest second refusal stands
	)
	a := New(mock, "m")
	a.Effort = EffortHigh
	ans, err := a.Ask(context.Background(), sampleDoc(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "does not specify") {
		t.Errorf("answer = %q (post-redirect refusal must be returned, not re-redirected)", ans.Text)
	}
	if mock.CallCount() != 3 {
		t.Fatalf("calls = %d want 3", mock.CallCount())
	}
}

func TestAskAgenticNoRedirectOnFinalBudgetTurn(t *testing.T) {
	// Fill every turn but the last with fetches; the last in-budget turn
	// refuses. There is no room left to explore, so no redirect — the refusal
	// is returned without consuming the forced-answer path.
	rs := make([]llm.MockResponse, 0, agentMaxIterations)
	for i := 0; i < agentMaxIterations-1; i++ {
		rs = append(rs, llm.MockResponse{Content: agentGetP2})
	}
	rs = append(rs, llm.MockResponse{Content: agentRefusal})
	mock := llm.NewMock("m", rs...)
	a := New(mock, "m")
	a.Effort = EffortHigh
	ans, err := a.Ask(context.Background(), sampleDoc(), "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "does not specify") {
		t.Errorf("answer = %q", ans.Text)
	}
	if mock.CallCount() != agentMaxIterations {
		t.Fatalf("calls = %d want %d (no redirect, no forced turn)", mock.CallCount(), agentMaxIterations)
	}
}
