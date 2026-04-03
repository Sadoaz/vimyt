package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Sadoaz/vimyt/internal/model"
	"github.com/Sadoaz/vimyt/internal/youtube"
)

// searchResultMsg carries results back from async search.
type searchResultMsg struct {
	query  string
	tracks []model.Track
	err    error
}

type searchModel struct {
	input       textinput.Model
	results     []model.Track
	cursor      int
	scroll      int  // viewport scroll offset
	visual      bool // whether visual-select mode is active
	anchor      int  // start of visual selection (cursor is the other end)
	loading     bool
	hasSearched bool // true after first submit(), prevents "No results found" while typing
	spinner     spinner.Model
	width       int
	height      int

	// Filter state (filter within search results)
	filterInput    textinput.Model
	filtering      bool
	filterQuery    string
	filteredResult []model.Track
	filteredIdx    []int // maps filtered index -> original results index

	// Favorites set (track IDs) — set by App before rendering
	favSet map[string]bool
	// Tick counter for marquee animation — set by App before rendering
	tick int
	// Whether this panel is currently focused — set by App before rendering
	focused bool
	// Whether to show relative line numbers — set by App before rendering
	relNumbers bool
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Placeholder = "Search"
	ti.CharLimit = 120

	fi := textinput.New()
	fi.CharLimit = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return searchModel{
		input:       ti,
		filterInput: fi,
		spinner:     sp,
	}
}

func (m searchModel) Init() tea.Cmd {
	return nil
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case searchResultMsg:
		m.loading = false
		if msg.err == nil {
			m.results = msg.tracks
			m.cursor = 0
			m.visual = false
			m.clearFilter()
			// Persist to disk so session restore doesn't re-fetch from YouTube
			if msg.query != "" && len(msg.tracks) > 0 {
				model.SaveSearchCache(msg.query, msg.tracks)
			}
		}
		// Return to normal mode so j/k navigate results
		m.input.Blur()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// If input is focused, handle text input
	if m.input.Focused() {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *searchModel) focus() tea.Cmd {
	return m.input.Focus()
}

func (m *searchModel) blur() {
	m.input.Blur()
}

// --- Filter ---

func (m *searchModel) startFilter() tea.Cmd {
	m.filterInput.SetValue("")
	m.filterInput.Placeholder = "Filter results..."
	return m.filterInput.Focus()
}

func (m *searchModel) liveFilter() {
	query := strings.TrimSpace(m.filterInput.Value())
	m.filterQuery = query

	if query == "" {
		m.filteredResult = nil
		m.filteredIdx = nil
		m.filterQuery = ""
		m.cursor = 0
		return
	}

	m.filteredResult = nil
	m.filteredIdx = nil
	for i, t := range m.results {
		if fuzzyMatch(t.Title, t.Artist, query) {
			m.filteredResult = append(m.filteredResult, t)
			m.filteredIdx = append(m.filteredIdx, i)
		}
	}
	m.cursor = 0
	m.visual = false
}

func (m *searchModel) confirmFilter() {
	m.liveFilter()
	m.filterInput.Blur()
}

func (m *searchModel) clearFilter() {
	m.filterQuery = ""
	m.filteredResult = nil
	m.filteredIdx = nil
}

func (m *searchModel) isFiltered() bool {
	return m.filterQuery != ""
}

func (m *searchModel) isFilterActive() bool {
	return m.filterInput.Focused()
}

// visibleTracks returns the filtered or full result list.
func (m *searchModel) visibleTracks() []model.Track {
	if m.isFiltered() {
		return m.filteredResult
	}
	return m.results
}

// realIndex maps a visible-list index to the actual results index.
func (m *searchModel) realIndex(visibleIdx int) int {
	if m.isFiltered() && visibleIdx < len(m.filteredIdx) {
		return m.filteredIdx[visibleIdx]
	}
	return visibleIdx
}

func (m *searchModel) submit() tea.Cmd {
	query := m.input.Value()
	if strings.TrimSpace(query) == "" {
		return nil
	}
	m.loading = true
	m.hasSearched = true
	m.input.Blur()
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			tracks, err := youtube.Search(query)
			return searchResultMsg{query: query, tracks: tracks, err: err}
		},
	)
}

func (m *searchModel) ensureVisible() {
	h := m.height
	if h < 1 {
		h = 10
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+h {
		m.scroll = m.cursor - h + 1
	}
	m.scroll = max(m.scroll, 0)
}

func (m *searchModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.ensureVisible()
	}
}

func (m *searchModel) moveDown() {
	if m.cursor < len(m.visibleTracks())-1 {
		m.cursor++
		m.ensureVisible()
	}
}

func (m *searchModel) goTop() {
	m.cursor = 0
	m.scroll = 0
}

func (m *searchModel) goBottom() {
	tracks := m.visibleTracks()
	if len(tracks) > 0 {
		m.cursor = len(tracks) - 1
		m.ensureVisible()
	}
}

func (m *searchModel) halfPageDown(visibleHeight int) {
	half := visibleHeight / 2
	if half < 1 {
		half = 1
	}
	m.cursor += half
	tracks := m.visibleTracks()
	m.cursor = min(m.cursor, len(tracks)-1)
	m.cursor = max(m.cursor, 0)
	m.ensureVisible()
}

func (m *searchModel) halfPageUp(visibleHeight int) {
	half := visibleHeight / 2
	if half < 1 {
		half = 1
	}
	m.cursor -= half
	m.cursor = max(m.cursor, 0)
	m.ensureVisible()
}

func (m *searchModel) toggleVisual() {
	m.visual = !m.visual
	if m.visual {
		m.anchor = m.cursor
	}
}

func (m *searchModel) swapVisualEnd() {
	if m.visual {
		m.anchor, m.cursor = m.cursor, m.anchor
	}
}

func (m *searchModel) isSelected(i int) bool {
	if !m.visual {
		return false
	}
	lo, hi := m.anchor, m.cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return i >= lo && i <= hi
}

func (m *searchModel) yankSelected() []model.Track {
	visible := m.visibleTracks()
	var tracks []model.Track
	if m.visual {
		lo, hi := m.anchor, m.cursor
		if lo > hi {
			lo, hi = hi, lo
		}
		for i := lo; i <= hi && i < len(visible); i++ {
			tracks = append(tracks, visible[i])
		}
	} else if len(visible) > 0 && m.cursor < len(visible) {
		tracks = append(tracks, visible[m.cursor])
	}
	m.visual = false
	return tracks
}

func (m *searchModel) currentTrack() *model.Track {
	visible := m.visibleTracks()
	if m.cursor < 0 || m.cursor >= len(visible) {
		return nil
	}
	t := visible[m.cursor]
	return &t
}

// fuzzyMatch checks if all whitespace-separated words in query appear
// somewhere in the combined title+artist string (case-insensitive).
// This allows matching "dream theater" against a track where "dream" is in the
// title and "theater" is in the artist, or words appearing in any order.
func fuzzyMatch(title, artist, query string) bool {
	combined := strings.ToLower(title + " " + artist)
	words := strings.Fields(strings.ToLower(query))
	for _, w := range words {
		if !strings.Contains(combined, w) {
			return false
		}
	}
	return len(words) > 0
}

// marquee scrolls a styled string horizontally if it exceeds maxW.
// tick drives the animation (1 tick = 1 scroll step).
// Pauses at the start and end of the scroll cycle.
func marquee(styled string, maxW int, tick int) string {
	textW := lipgloss.Width(styled)
	if textW <= maxW {
		return styled
	}
	overflow := textW - maxW
	pause := 4 // ticks to pause at each end
	cycle := pause + overflow + pause
	pos := tick % cycle
	var offset int
	if pos < pause {
		offset = 0
	} else if pos < pause+overflow {
		offset = pos - pause
	} else {
		offset = overflow
	}
	shifted := ansi.TruncateLeft(styled, offset, "")
	return ansi.Truncate(shifted, maxW, "")
}

var (
	searchInputStyle    = lipgloss.NewStyle().Padding(0, 1)
	resultNormalStyle   = lipgloss.NewStyle().Padding(0, 2)
	resultSelectedStyle = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("238"))
	resultCursorStyle   = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("238"))
	resultBothStyle     = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("238")).Bold(true)
	durationStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	artistStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	favStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
)

func (m searchModel) View() string {
	return m.ViewConstrained(m.width, m.height)
}

var searchFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)

func (m searchModel) ViewConstrained(width, height int) string {
	var b strings.Builder

	// Search input
	b.WriteString(searchInputStyle.Render(m.input.View()))
	b.WriteString("\n\n")

	if m.loading {
		fmt.Fprintf(&b, "  %s Searching...", m.spinner.View())
		return b.String()
	}

	if len(m.results) == 0 && m.hasSearched && m.input.Value() != "" {
		b.WriteString("  No results found")
		return b.String()
	}

	// Filter indicator
	filterLines := 0
	if m.isFiltered() {
		b.WriteString(searchFilterStyle.Render(fmt.Sprintf("  (filter: %s)", m.filterQuery)))
		b.WriteString("\n\n")
		filterLines = 2
	}

	tracks := m.visibleTracks()

	if len(tracks) == 0 && m.isFiltered() {
		b.WriteString("  No matching tracks.")
		return b.String()
	}

	// Results list
	maxVisible := height - 2 - filterLines // subtract input line + blank line + filter
	if maxVisible < 1 {
		maxVisible = 10
	}

	// Adjust scroll to keep cursor visible
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+maxVisible {
		m.scroll = m.cursor - maxVisible + 1
	}
	m.scroll = min(m.scroll, len(tracks)-maxVisible)
	m.scroll = max(m.scroll, 0)
	start := m.scroll
	end := min(start+maxVisible, len(tracks))

	for i := start; i < end; i++ {
		t := tracks[i]
		// Show the real result number, not the filtered index
		realIdx := m.realIndex(i)
		dur := formatDuration(t.Duration)
		heart := ""
		if m.favSet[t.ID] {
			heart = favStyle.Render(" <3")
		}
		lineNum := realIdx + 1
		if m.relNumbers && i != m.cursor {
			dist := i - m.cursor
			if dist < 0 {
				dist = -dist
			}
			lineNum = dist
		}
		isCursor := i == m.cursor && m.focused
		isSel := m.isSelected(i) && m.focused

		style := resultNormalStyle
		prefix := "  "
		switch {
		case isCursor && isSel:
			style = resultBothStyle
			prefix = "> "
		case isCursor:
			style = resultCursorStyle
			prefix = "> "
		case isSel:
			style = resultSelectedStyle
		}

		var line string
		hasBackground := isCursor || isSel
		if hasBackground {
			line = fmt.Sprintf("%2d  %s  %s  %s",
				lineNum, t.Title, t.Artist, dur)
		} else {
			line = fmt.Sprintf("%2d  %s  %s  %s",
				lineNum,
				t.Title,
				artistStyle.Render(t.Artist),
				durationStyle.Render(dur),
			)
		}
		content := prefix + line + heart
		if isSel && !isCursor && width > 0 {
			content = ansi.Truncate(content, width-4, "")
		}
		rendered := style.Render(content)
		if isCursor && width > 0 {
			rendered = marquee(rendered, width, m.tick)
		} else if width > 0 {
			rendered = ansi.Truncate(rendered, width, "")
		}
		b.WriteString(rendered)
		b.WriteString("\n")
	}

	return b.String()
}
