package llm

import (
	"context"
	"sync"
)

// MockResponse is one scripted reply (or error) for MockProvider.
type MockResponse struct {
	Content string
	Finish  string
	Err     error
}

// MockProvider is a deterministic in-memory Provider for tests and the cassette
// seam. It serves Responses in order, then falls back to Default, and records
// every request it receives.
type MockProvider struct {
	mu        sync.Mutex
	responses []MockResponse
	Default   MockResponse
	name      string
	calls     []Request
	idx       int
}

// NewMock builds a MockProvider that returns the given responses in order.
func NewMock(name string, responses ...MockResponse) *MockProvider {
	return &MockProvider{name: name, responses: responses}
}

// FailThenSucceed returns a mock that fails `failures` times with err, then
// returns content. Handy for exercising the retry layer.
func FailThenSucceed(failures int, err error, content string) *MockProvider {
	rs := make([]MockResponse, 0, failures+1)
	for i := 0; i < failures; i++ {
		rs = append(rs, MockResponse{Err: err})
	}
	rs = append(rs, MockResponse{Content: content, Finish: "stop"})
	return NewMock("flaky", rs...)
}

// Name implements Provider.
func (m *MockProvider) Name() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}

// Complete implements Provider.
func (m *MockProvider) Complete(_ context.Context, req Request) (Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, req)
	r := m.Default
	if m.idx < len(m.responses) {
		r = m.responses[m.idx]
		m.idx++
	}
	if r.Err != nil {
		return Response{}, r.Err
	}
	return Response{Content: r.Content, FinishReason: r.Finish}, nil
}

// CallCount returns how many times Complete was called.
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// Calls returns a copy of the recorded requests.
func (m *MockProvider) Calls() []Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Request(nil), m.calls...)
}
