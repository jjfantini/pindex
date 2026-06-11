package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
)

// NewLogger returns a charmbracelet/log logger bound to the UI's writer and
// color mode. It is the under-the-hood diagnostic channel: LLM call timings,
// cache hits, retries, and breaker events land here so an operator (human or
// agent) can see exactly what the engine is doing. verbose enables the debug
// level; otherwise only info and above are shown.
func (u *UI) NewLogger(verbose bool) *log.Logger {
	l := log.NewWithOptions(u.w, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
	})
	if u.plain {
		l.SetColorProfile(termenv.Ascii)
	}
	if verbose {
		l.SetLevel(log.DebugLevel)
	} else {
		l.SetLevel(log.InfoLevel)
	}

	st := log.DefaultStyles()
	st.Timestamp = st.Timestamp.Foreground(colorDim)
	st.Key = lipgloss.NewStyle().Foreground(colorAccent)
	st.Levels[log.DebugLevel] = st.Levels[log.DebugLevel].Foreground(colorDim)
	st.Levels[log.InfoLevel] = st.Levels[log.InfoLevel].Foreground(colorBrand)
	st.Levels[log.WarnLevel] = st.Levels[log.WarnLevel].Foreground(colorWarn)
	st.Levels[log.ErrorLevel] = st.Levels[log.ErrorLevel].Foreground(colorError)
	l.SetStyles(st)
	return l
}
