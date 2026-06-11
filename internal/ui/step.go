package ui

import (
	"fmt"
	"sync"
	"time"
)

// spinnerFrames is the braille spinner cycle used while a step is running.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 90 * time.Millisecond

// Step is one long-running unit of work ("extracting pages", "building tree
// index"). Animated mode redraws a single spinner line in place; plain mode
// emits a start line, one line per Update milestone, and a final ✓/✗ line —
// so a piped consumer sees the same iterative progress without control codes.
// All methods are safe for concurrent use.
type Step struct {
	u        *UI
	label    string
	start    time.Time
	mu       sync.Mutex
	detail   string
	finished bool
	stopCh   chan struct{}
	doneWg   sync.WaitGroup
}

// Step starts a new step labeled label.
func (u *UI) Step(label string) *Step {
	s := &Step{u: u, label: label, start: time.Now()}
	if u.animate {
		s.stopCh = make(chan struct{})
		s.doneWg.Add(1)
		go s.spin()
		return s
	}
	_, _ = fmt.Fprintln(u.w, u.st.IconInfo+" "+label+u.st.Dim.Render("…"))
	return s
}

// Update sets the step's current detail (e.g. "structure group 2/5"). Animated
// mode shows it on the spinner line; plain mode prints it as a milestone line.
func (s *Step) Update(format string, args ...any) {
	detail := fmt.Sprintf(format, args...)
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.detail = detail
	animate := s.u.animate
	s.mu.Unlock()
	if !animate {
		_, _ = fmt.Fprintln(s.u.w, "  "+s.u.st.Dim.Render("› "+detail))
	}
}

// Done finishes the step with a green check. An empty message reuses the label.
func (s *Step) Done(format string, args ...any) {
	s.finish(s.u.st.IconOK, fmt.Sprintf(orLabel(format, s.label), args...))
}

// Fail finishes the step with a red cross. An empty message reuses the label.
func (s *Step) Fail(format string, args ...any) {
	s.finish(s.u.st.IconErr, fmt.Sprintf(orLabel(format, s.label), args...))
}

func orLabel(format, label string) string {
	if format == "" {
		return label
	}
	return format
}

func (s *Step) finish(icon, msg string) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.mu.Unlock()
	if s.u.animate {
		close(s.stopCh)
		s.doneWg.Wait()
		_, _ = fmt.Fprint(s.u.w, "\r\x1b[2K")
	}
	elapsed := s.u.st.Dim.Render("(" + fmtDur(time.Since(s.start)) + ")")
	_, _ = fmt.Fprintln(s.u.w, icon+" "+msg+" "+elapsed)
}

func (s *Step) spin() {
	defer s.doneWg.Done()
	t := time.NewTicker(spinnerInterval)
	defer t.Stop()
	for i := 0; ; i++ {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			s.mu.Lock()
			line := s.u.st.Spinner.Render(spinnerFrames[i%len(spinnerFrames)]) + " " + s.label
			if s.detail != "" {
				line += " " + s.u.st.Dim.Render("› "+s.detail)
			}
			s.mu.Unlock()
			line += " " + s.u.st.Dim.Render("("+fmtDur(time.Since(s.start))+")")
			_, _ = fmt.Fprint(s.u.w, "\r\x1b[2K"+line)
		}
	}
}

// fmtDur renders a duration at human precision: 420ms, 1.2s, 1m5s.
func fmtDur(d time.Duration) string {
	switch {
	case d < time.Second:
		return d.Round(10 * time.Millisecond).String()
	case d < time.Minute:
		return d.Round(100 * time.Millisecond).String()
	default:
		return d.Round(time.Second).String()
	}
}
