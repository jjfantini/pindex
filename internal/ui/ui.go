// Package ui is pindex's terminal presentation layer: lipgloss-styled,
// non-interactive output that adapts to where it is written. On a TTY it
// colors and animates (an in-place spinner); when piped — CI, agents,
// sandboxed terminals — it degrades to pure line-oriented output with no
// control codes, so the CLI never garbles a machine-read stream and never
// blocks waiting for input. All ui output belongs on stderr; stdout payload
// contracts (JSON trees, answer text, extracted pages) are never touched here.
//
// Mode resolution: --plain / PINDEX_PLAIN=1 force plain mode; otherwise the
// writer must be a real terminal (and TERM != dumb) to animate. NO_COLOR is
// honored by the underlying color profile detection.
package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

// UI renders styled status output to a single writer (conventionally stderr).
type UI struct {
	w       io.Writer
	re      *lipgloss.Renderer
	st      Styles
	animate bool // in-place spinner redraws (TTY only)
	plain   bool // line-oriented, uncolored output
}

// Option configures a UI.
type Option func(*UI)

// Plain forces line-oriented, uncolored output regardless of the writer.
func Plain() Option {
	return func(u *UI) { u.plain = true }
}

// NoAnimation keeps styling but disables in-place redraws. Used with
// --verbose, where streaming log lines would fight the spinner for the line.
func NoAnimation() Option {
	return func(u *UI) { u.animate = false }
}

// ForceAnimate enables TTY behavior (colors + spinner frames) regardless of
// the writer. Test hook — real callers rely on auto-detection.
func ForceAnimate() Option {
	return func(u *UI) {
		u.plain = false
		u.animate = true
		u.re.SetColorProfile(termenv.ANSI256)
	}
}

// New returns a UI bound to w, auto-detecting TTY vs plain mode.
func New(w io.Writer, opts ...Option) *UI {
	u := &UI{w: w, re: lipgloss.NewRenderer(w)}
	u.animate = isTerminal(w) && os.Getenv("TERM") != "dumb"
	if os.Getenv("PINDEX_PLAIN") != "" {
		u.plain = true
	}
	for _, o := range opts {
		o(u)
	}
	if u.plain {
		u.animate = false
		u.re.SetColorProfile(termenv.Ascii)
	}
	u.st = newStyles(u.re)
	return u
}

// Animated reports whether the UI redraws in place (a real, non-dumb TTY).
func (u *UI) Animated() bool { return u.animate }

// Writer returns the writer ui output goes to.
func (u *UI) Writer() io.Writer { return u.w }

// Styles exposes the resolved style set (for callers composing custom lines).
func (u *UI) Styles() Styles { return u.st }

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && (isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd()))
}

// Printf writes formatted text to the UI writer (no styling).
func (u *UI) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(u.w, format, args...)
}

// Println writes a line to the UI writer (no styling).
func (u *UI) Println(args ...any) {
	_, _ = fmt.Fprintln(u.w, args...)
}

// Successf prints a green check line: "✓ msg".
func (u *UI) Successf(format string, args ...any) {
	u.statusLine(u.st.IconOK, format, args...)
}

// Warnf prints an amber warning line: "⚠ msg".
func (u *UI) Warnf(format string, args ...any) {
	u.statusLine(u.st.IconWarn, format, args...)
}

// Errorf prints a red cross line: "✗ msg".
func (u *UI) Errorf(format string, args ...any) {
	u.statusLine(u.st.IconErr, format, args...)
}

// Infof prints a neutral bullet line: "• msg".
func (u *UI) Infof(format string, args ...any) {
	u.statusLine(u.st.IconInfo, format, args...)
}

// Notef prints a dim secondary line: "› msg" (paths, hints, context).
func (u *UI) Notef(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintln(u.w, u.st.Dim.Render("› "+msg))
}

func (u *UI) statusLine(icon string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintln(u.w, icon+" "+msg)
}

// Header prints the command banner: a brand badge, the verb, and its subject.
// Example: " pindex  index · testdata/sample.pdf".
func (u *UI) Header(verb, subject string) {
	line := u.st.Brand.Render(" pindex ") + " " + u.st.Title.Render(verb)
	if subject != "" {
		line += " " + u.st.Dim.Render("· "+subject)
	}
	_, _ = fmt.Fprintln(u.w, line)
}
