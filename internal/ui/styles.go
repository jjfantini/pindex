package ui

import "github.com/charmbracelet/lipgloss"

// Palette: the charm-purple brand plus adaptive status colors that read well
// on both light and dark terminals. Colors degrade automatically with the
// renderer's profile (Ascii when piped, honoring NO_COLOR).
var (
	colorBrand   = lipgloss.AdaptiveColor{Light: "#5A3FD4", Dark: "#7D56F4"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "#02875D", Dark: "#04B575"}
	colorWarn    = lipgloss.AdaptiveColor{Light: "#B58900", Dark: "#F2C94C"}
	colorError   = lipgloss.AdaptiveColor{Light: "#D70000", Dark: "#FF5F87"}
	colorDim     = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	colorAccent  = lipgloss.AdaptiveColor{Light: "#0087AF", Dark: "#5FD7FF"}
)

// Styles is the resolved style set for one renderer (writer-bound, so color
// capability is decided per destination, never globally).
type Styles struct {
	Brand   lipgloss.Style // " pindex " badge: white on brand purple
	Title   lipgloss.Style // command verb, bold
	Dim     lipgloss.Style // secondary text: paths, hints, durations
	Accent  lipgloss.Style // highlighted values (counts, models, ids)
	Success lipgloss.Style
	Warn    lipgloss.Style
	Error   lipgloss.Style
	Spinner lipgloss.Style // spinner frame glyph

	BoxBorder    lipgloss.Style // rounded summary box
	BoxTitle     lipgloss.Style
	TableHeader  lipgloss.Style
	TableBorder  lipgloss.Style
	TreeRoot     lipgloss.Style
	TreeItem     lipgloss.Style
	TreeBranches lipgloss.Style

	// Pre-rendered status icons (styled glyph, ready to prepend).
	IconOK   string
	IconErr  string
	IconWarn string
	IconInfo string
	IconSkip string
}

func newStyles(re *lipgloss.Renderer) Styles {
	s := Styles{
		Brand:   re.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(colorBrand),
		Title:   re.NewStyle().Bold(true),
		Dim:     re.NewStyle().Foreground(colorDim),
		Accent:  re.NewStyle().Foreground(colorAccent),
		Success: re.NewStyle().Foreground(colorSuccess),
		Warn:    re.NewStyle().Foreground(colorWarn),
		Error:   re.NewStyle().Foreground(colorError),
		Spinner: re.NewStyle().Foreground(colorBrand),

		BoxBorder:    re.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBrand).Padding(0, 1),
		BoxTitle:     re.NewStyle().Bold(true).Foreground(colorBrand),
		TableHeader:  re.NewStyle().Bold(true).Foreground(colorBrand).Padding(0, 1),
		TableBorder:  re.NewStyle().Foreground(colorDim),
		TreeRoot:     re.NewStyle().Bold(true).Foreground(colorBrand),
		TreeItem:     re.NewStyle(),
		TreeBranches: re.NewStyle().Foreground(colorDim),
	}
	s.IconOK = s.Success.Render("✓")
	s.IconErr = s.Error.Render("✗")
	s.IconWarn = s.Warn.Render("⚠")
	s.IconInfo = s.Dim.Render("•")
	s.IconSkip = s.Dim.Render("⊘")
	return s
}
