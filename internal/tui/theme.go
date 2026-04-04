package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// activeTheme is the current theme, accessible from anywhere in the package.
var activeTheme = DefaultTheme()

// Theme holds all customizable colors for the TUI.
type Theme struct {
	// Cursor / selection background
	CursorBg string `json:"cursor_bg"`
	// Accent color for focused borders, labels, key hints
	Accent string `json:"accent"`
	// Secondary/dimmed text (metadata, descriptions, counts)
	Dimmed string `json:"dimmed"`
	// Muted text (filters, time display, status bar)
	Muted string `json:"muted"`
	// Unfocused panel border and label color
	Unfocused string `json:"unfocused"`
	// Disabled/off state text
	Disabled string `json:"disabled"`
	// Currently playing track indicator
	Playing string `json:"playing"`
	// Enabled/on state text
	Enabled string `json:"enabled"`
	// Now-playing title (track name)
	NowPlayingTitle string `json:"now_playing_title"`
	// Now-playing artist name
	NowPlayingArtist string `json:"now_playing_artist"`
	// Artist name in search/history results
	ArtistText string `json:"artist_text"`
	// Paused state indicator
	Paused string `json:"paused"`
	// Favorite heart icon
	Favorite string `json:"favorite"`
	// Radio mode badge
	Radio string `json:"radio"`
	// Create new playlist action
	Create string `json:"create"`
	// Progress bar filled portion
	BarFilled string `json:"bar_filled"`
	// Progress bar unfilled portion
	BarEmpty string `json:"bar_empty"`
}

// DefaultTheme returns the default color theme.
func DefaultTheme() Theme {
	return Theme{
		CursorBg:         "#444444",
		Accent:           "#5fafff",
		Dimmed:           "#767676",
		Muted:            "#8a8a8a",
		Unfocused:        "#585858",
		Disabled:         "#626262",
		Playing:          "#00ff87",
		Enabled:          "#5fff00",
		NowPlayingTitle:  "#ffffff",
		NowPlayingArtist: "#9e9e9e",
		ArtistText:       "#bcbcbc",
		Paused:           "#ff8700",
		Favorite:         "#ff5f87",
		Radio:            "#ff87d7",
		Create:           "#ffaf00",
		BarFilled:        "#5fafff",
		BarEmpty:         "#444444",
	}
}

// themeFields returns the theme fields as an ordered list for the settings UI.
var themeFields = []struct {
	name string
	desc string
	get  func(*Theme) string
	set  func(*Theme, string)
}{
	{"Cursor BG", "Cursor/selection background", func(t *Theme) string { return t.CursorBg }, func(t *Theme, v string) { t.CursorBg = v }},
	{"Accent", "Focused borders, labels, hints", func(t *Theme) string { return t.Accent }, func(t *Theme, v string) { t.Accent = v }},
	{"Dimmed", "Secondary text, metadata", func(t *Theme) string { return t.Dimmed }, func(t *Theme, v string) { t.Dimmed = v }},
	{"Muted", "Filters, time display", func(t *Theme) string { return t.Muted }, func(t *Theme, v string) { t.Muted = v }},
	{"Unfocused", "Unfocused panel borders", func(t *Theme) string { return t.Unfocused }, func(t *Theme, v string) { t.Unfocused = v }},
	{"Disabled", "Off state text", func(t *Theme) string { return t.Disabled }, func(t *Theme, v string) { t.Disabled = v }},
	{"Playing", "Currently playing track", func(t *Theme) string { return t.Playing }, func(t *Theme, v string) { t.Playing = v }},
	{"Enabled", "On state text", func(t *Theme) string { return t.Enabled }, func(t *Theme, v string) { t.Enabled = v }},
	{"NP Title", "Now-playing title", func(t *Theme) string { return t.NowPlayingTitle }, func(t *Theme, v string) { t.NowPlayingTitle = v }},
	{"NP Artist", "Now-playing artist", func(t *Theme) string { return t.NowPlayingArtist }, func(t *Theme, v string) { t.NowPlayingArtist = v }},
	{"Artist", "Artist name in lists", func(t *Theme) string { return t.ArtistText }, func(t *Theme, v string) { t.ArtistText = v }},
	{"Paused", "Paused indicator", func(t *Theme) string { return t.Paused }, func(t *Theme, v string) { t.Paused = v }},
	{"Favorite", "Heart icon color", func(t *Theme) string { return t.Favorite }, func(t *Theme, v string) { t.Favorite = v }},
	{"Radio", "Radio mode badge", func(t *Theme) string { return t.Radio }, func(t *Theme, v string) { t.Radio = v }},
	{"Create", "Create new action", func(t *Theme) string { return t.Create }, func(t *Theme, v string) { t.Create = v }},
	{"Bar Filled", "Progress bar filled", func(t *Theme) string { return t.BarFilled }, func(t *Theme, v string) { t.BarFilled = v }},
	{"Bar Empty", "Progress bar unfilled", func(t *Theme) string { return t.BarEmpty }, func(t *Theme, v string) { t.BarEmpty = v }},
}

// ToMap converts the theme to a map for session persistence.
func (t Theme) ToMap() map[string]string {
	def := DefaultTheme()
	m := make(map[string]string)
	for _, f := range themeFields {
		val := f.get(&t)
		defVal := f.get(&def)
		if val != defVal {
			m[f.name] = val
		}
	}
	return m
}

// ThemeFromMap creates a theme by applying overrides to the default.
func ThemeFromMap(m map[string]string) Theme {
	t := DefaultTheme()
	if m == nil {
		return t
	}
	for _, f := range themeFields {
		if val, ok := m[f.name]; ok && val != "" {
			f.set(&t, val)
		}
	}
	return t
}

// colorFilteredFields returns indices of themeFields matching the filter.
func (a *App) colorFilteredFields() []int {
	if a.colorFilter == "" {
		indices := make([]int, len(themeFields))
		for i := range themeFields {
			indices[i] = i
		}
		return indices
	}
	filter := strings.ToLower(a.colorFilter)
	var indices []int
	for i, f := range themeFields {
		if strings.Contains(strings.ToLower(f.name), filter) ||
			strings.Contains(strings.ToLower(f.desc), filter) {
			indices = append(indices, i)
		}
	}
	return indices
}

// applyTheme rebuilds all lipgloss styles from the current theme.
func applyTheme(t Theme) {
	activeTheme = t
	// Cursor / selection backgrounds
	cursorBg := lipgloss.Color(t.CursorBg)
	artistCurStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	radioCurStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	settingsCurStyle = lipgloss.NewStyle().Background(cursorBg)

	// Accent
	accent := lipgloss.Color(t.Accent)
	npSettingStyle = lipgloss.NewStyle().Foreground(accent)
	helpKeyStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	helpHeaderStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)

	// Dimmed
	dimmed := lipgloss.Color(t.Dimmed)
	artistDimStyle = lipgloss.NewStyle().Foreground(dimmed)
	helpDescStyle = lipgloss.NewStyle().Foreground(dimmed)

	// Disabled
	disabled := lipgloss.Color(t.Disabled)
	settingsOffStyle = lipgloss.NewStyle().Foreground(disabled)
	helpDimStyle = lipgloss.NewStyle().Foreground(disabled)

	// Enabled
	settingsOnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Enabled)).Bold(true)

	// Now-playing
	npTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.NowPlayingTitle))
	npArtistStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.NowPlayingArtist))
	npTimeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))
	npPausedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Paused))

	// Favorite
	favStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Favorite))

	// Status bar
	sbBgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))

	// Queue styles
	queueNormalStyle = lipgloss.NewStyle().Padding(0, 2)
	queueCursorStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	queuePlayingStyle = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color(t.Playing)).Bold(true)
	queueBothStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg).Foreground(lipgloss.Color(t.Playing)).Bold(true)
	queueSelStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	queueSelCurStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg).Bold(true)
	queueFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))

	// Search styles
	resultNormalStyle = lipgloss.NewStyle().Padding(0, 2)
	resultSelectedStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	resultCursorStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	resultBothStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg).Bold(true)
	durationStyle = lipgloss.NewStyle().Foreground(dimmed)
	artistStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.ArtistText))
	searchFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))

	// Playlist styles
	plNormalStyle = lipgloss.NewStyle().Padding(0, 2)
	plCursorStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	plSelStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	plBothStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg).Bold(true)
	plCountStyle = lipgloss.NewStyle().Foreground(dimmed)
	plRadioStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Radio)).Bold(true)
	plFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted))

	// Playlist overlay styles
	overlayBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Accent)).
		Padding(1, 2)
	overlayTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true)
	overlayCurStyle = lipgloss.NewStyle().Background(cursorBg)
	overlayQueueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Playing)).Bold(true)
	overlayCreateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Create)).Bold(true)

	// History styles
	histNormalStyle = lipgloss.NewStyle().Padding(0, 2)
	histCursorStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	histSelStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg)
	histBothStyle = lipgloss.NewStyle().Padding(0, 2).Background(cursorBg).Bold(true)

	// Artist normal style
	artistNormalStyle = lipgloss.NewStyle().Padding(0, 2)

	// Radio normal style
	radioNormalStyle = lipgloss.NewStyle().Padding(0, 2)
}
