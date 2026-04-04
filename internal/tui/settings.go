package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Sadoaz/vimyt/internal/youtube"
)

// settingsOptions defines the available settings in order.
var settingsOptions = []struct {
	name string
	desc string
}{
	{"Autoplay", "Auto-advance to next track when current ends"},        // 0
	{"Shuffle", "Randomize next track selection"},                       // 1
	{"Loop Track", "Replay current track (∞ or x times)"},               // 2
	{"Focus Queue", "Auto-focus queue panel when playing a track"},      // 3
	{"Rel Numbers", "Show relative line numbers (vim-style)"},           // 4
	{"Pin Search", "Keep search panel expanded when unfocused"},         // 5
	{"Pin Playlist", "Keep playlist detail expanded when unfocused"},    // 6
	{"Pin Radio", "Keep radio history expanded when unfocused"},         // 7
	{"Pin Artists", "Keep artists panel expanded when unfocused"},       // 8
	{"Show History", "Show play history panel below playlists"},         // 9
	{"Show Radio", "Show radio history panel below play history"},       // 10
	{"Show Artists", "Show artists panel"},                              // 11
	{"Colors", "Customize TUI colors"},                                  // 12
	{"YT Auth", "Use browser cookies to access your private playlists"}, // 13
	{"Import", "Import playlist from YouTube URL"},                      // 14
}

// browserOptions is the cycle for the Auth Browser setting.
var browserOptions = []string{"", "firefox", "chrome", "chromium", "brave", "edge"}

func (a *App) settingValue(idx int) bool {
	switch idx {
	case 0:
		return a.autoplay
	case 1:
		return a.shuffle
	case 2:
		return a.loopTrack
	case 3:
		return a.autoFocusQueue
	case 4:
		return a.relNumbers
	case 5:
		return a.pinSearch
	case 6:
		return a.pinPlaylist
	case 7:
		return a.pinRadio
	case 8:
		return a.pinArtists
	case 9:
		return a.showHistory
	case 10:
		return a.showRadio
	case 11:
		return a.showArtistsPanel
	case 12:
		return false // Colors — not a boolean toggle
	case 13:
		return a.cookieBrowser != ""
	}
	return false
}

func (a *App) toggleSetting(idx int) {
	switch idx {
	case 0:
		a.autoplay = !a.autoplay
	case 1:
		a.shuffle = !a.shuffle
		if !a.shuffle {
			a.shufflePlayed = nil
		}
	case 2:
		// Loop Track: cycle Off → ∞ → input mode
		if !a.loopTrack {
			a.loopTrack = true
			a.loopCount = 0
			a.loopTotal = 0
		} else if a.loopTotal == 0 {
			a.settingsLoopInput = true
			a.settingsLoopInp.SetValue("")
			a.settingsLoopInp.Focus()
		} else {
			a.loopTrack = false
			a.loopCount = 0
			a.loopTotal = 0
		}
	case 3:
		a.autoFocusQueue = !a.autoFocusQueue
	case 4:
		a.relNumbers = !a.relNumbers
	case 5:
		a.pinSearch = !a.pinSearch
	case 6:
		a.pinPlaylist = !a.pinPlaylist
	case 7:
		a.pinRadio = !a.pinRadio
	case 8:
		a.pinArtists = !a.pinArtists
	case 9:
		a.showHistory = !a.showHistory
		if !a.showHistory && a.focusedPanel == panelHistory {
			a.focusedPanel = panelPlaylist
		}
	case 10:
		a.showRadio = !a.showRadio
		if !a.showRadio && a.focusedPanel == panelRadioHist {
			a.focusedPanel = panelPlaylist
		}
	case 11:
		a.showArtistsPanel = !a.showArtistsPanel
		if !a.showArtistsPanel && a.focusedPanel == panelArtists {
			a.focusedPanel = panelPlaylist
		}
	case 12:
		// Colors — opens color editor, handled in updateSettings
	case 13:
		a.cycleBrowser(1)
	}
}

func (a *App) loopOff() {
	a.loopTrack = false
	a.loopCount = 0
	a.loopTotal = 0
}

func (a *App) cycleBrowser(dir int) {
	cur := 0
	for i, b := range browserOptions {
		if b == a.cookieBrowser {
			cur = i
			break
		}
	}
	next := (cur + dir + len(browserOptions)) % len(browserOptions)
	a.cookieBrowser = browserOptions[next]
	youtube.SetCookieBrowser(a.cookieBrowser)
}

// settingsFilteredIndices returns the indices of settings matching the filter.
func (a *App) settingsFilteredIndices() []int {
	if a.settingsFilter == "" {
		indices := make([]int, len(settingsOptions))
		for i := range settingsOptions {
			indices[i] = i
		}
		return indices
	}
	filter := strings.ToLower(a.settingsFilter)
	var indices []int
	for i, opt := range settingsOptions {
		if strings.Contains(strings.ToLower(opt.name), filter) ||
			strings.Contains(strings.ToLower(opt.desc), filter) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (a App) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle color editor sub-view
	if a.showColorEditor {
		return a.updateColorEditor(msg)
	}

	// Handle settings search input
	if a.settingsSearching {
		switch {
		case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
			a.quit()
			return a, tea.Quit
		case key.Matches(msg, keys.Enter):
			a.settingsFilter = a.settingsFilterInp.Value()
			a.settingsSearching = false
			a.settingsFilterInp.Blur()
			a.settingsCur = 0
			return a, nil
		case key.Matches(msg, keys.Escape):
			a.settingsSearching = false
			a.settingsFilterInp.Blur()
			if a.settingsFilterInp.Value() == "" {
				a.settingsFilter = ""
			}
			return a, nil
		default:
			var cmd tea.Cmd
			a.settingsFilterInp, cmd = a.settingsFilterInp.Update(msg)
			return a, cmd
		}
	}

	// Handle import URL input mode
	if a.settingsImporting {
		switch {
		case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
			a.quit()
			return a, tea.Quit
		case key.Matches(msg, keys.Enter):
			url := strings.TrimSpace(a.settingsImportInput.Value())
			a.settingsImporting = false
			a.settingsImportInput.Blur()
			if url == "" {
				return a, nil
			}
			if a.importingPlaylist {
				return a, nil // already importing
			}
			a.importingPlaylist = true
			a.showSettings = false
			cmd := a.setStatus("Importing playlist...")
			importCmd := func() tea.Msg {
				name, tracks, err := youtube.FetchPlaylist(url)
				return importPlaylistMsg{name: name, tracks: tracks, err: err}
			}
			return a, tea.Batch(cmd, importCmd)
		case key.Matches(msg, keys.Escape):
			a.settingsImporting = false
			a.settingsImportInput.Blur()
			return a, nil
		default:
			var cmd tea.Cmd
			a.settingsImportInput, cmd = a.settingsImportInput.Update(msg)
			return a, cmd
		}
	}

	// Handle loop count input mode
	if a.settingsLoopInput {
		switch {
		case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
			a.quit()
			return a, tea.Quit
		case key.Matches(msg, keys.Enter):
			val := strings.TrimSpace(a.settingsLoopInp.Value())
			a.settingsLoopInput = false
			a.settingsLoopInp.Blur()
			if val == "" {
				return a, nil
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return a, nil
			}
			a.loopTrack = true
			a.loopCount = n
			a.loopTotal = n
			return a, nil
		case key.Matches(msg, keys.Escape):
			a.settingsLoopInput = false
			a.settingsLoopInp.Blur()
			return a, nil
		default:
			var cmd tea.Cmd
			a.settingsLoopInp, cmd = a.settingsLoopInp.Update(msg)
			return a, cmd
		}
	}

	filtered := a.settingsFilteredIndices()

	switch {
	case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case key.Matches(msg, keys.Escape), key.Matches(msg, keys.Settings), msg.String() == "q":
		if a.settingsFilter != "" {
			a.settingsFilter = ""
			a.settingsCur = 0
			return a, nil
		}
		a.showSettings = false
		return a, nil
	case key.Matches(msg, keys.Up):
		if a.settingsCur > 0 {
			a.settingsCur--
		}
		return a, nil
	case key.Matches(msg, keys.Down):
		if a.settingsCur < len(filtered)-1 {
			a.settingsCur++
		}
		return a, nil
	case key.Matches(msg, keys.HalfDown):
		a.settingsCur += len(filtered) / 2
		a.settingsCur = min(a.settingsCur, len(filtered)-1)
		return a, nil
	case key.Matches(msg, keys.HalfUp):
		a.settingsCur = max(a.settingsCur-len(filtered)/2, 0)
		return a, nil
	case msg.String() == "g":
		a.settingsCur = 0
		return a, nil
	case msg.String() == "G":
		a.settingsCur = len(filtered) - 1
		return a, nil
	case msg.String() == "/":
		a.settingsSearching = true
		a.settingsFilterInp.SetValue(a.settingsFilter)
		a.settingsFilterInp.Focus()
		return a, nil
	case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Space):
		if a.settingsCur >= len(filtered) {
			return a, nil
		}
		realIdx := filtered[a.settingsCur]
		if realIdx == 12 { // Colors
			a.showColorEditor = true
			a.colorEditorCur = 0
			return a, nil
		}
		if realIdx == 14 { // Import Playlist
			a.settingsImporting = true
			a.settingsImportInput.SetValue("")
			a.settingsImportInput.Focus()
			return a, nil
		}
		a.toggleSetting(realIdx)
		return a, nil
	case msg.String() == "l", msg.String() == "right":
		if a.settingsCur >= len(filtered) {
			return a, nil
		}
		realIdx := filtered[a.settingsCur]
		switch realIdx {
		case 2:
			a.toggleSetting(2)
		case 12:
			a.showColorEditor = true
			a.colorEditorCur = 0
		case 13:
			a.cycleBrowser(1)
		case 14:
		default:
			if !a.settingValue(realIdx) {
				a.toggleSetting(realIdx)
			}
		}
		return a, nil
	case msg.String() == "h", msg.String() == "left":
		if a.settingsCur >= len(filtered) {
			return a, nil
		}
		realIdx := filtered[a.settingsCur]
		switch realIdx {
		case 2:
			a.loopOff()
		case 12:
		case 13:
			a.cycleBrowser(-1)
		case 14:
		default:
			if a.settingValue(realIdx) {
				a.toggleSetting(realIdx)
			}
		}
		return a, nil
	}
	return a, nil
}

var (
	settingsOnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	settingsOffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	settingsCurStyle = lipgloss.NewStyle().Background(lipgloss.Color("238"))
)

func (a App) renderSettings() string {
	if a.showColorEditor {
		return a.renderColorEditor()
	}

	boxW := min(max(a.width*2/3, 50), a.width-2)
	innerW := boxW - 6 // border(2) + padding(4)
	if innerW < 20 {
		innerW = 20
	}
	// Available lines for settings items
	maxVisible := a.height - 9
	if maxVisible < 3 {
		maxVisible = 3
	}

	filtered := a.settingsFilteredIndices()
	total := len(filtered)

	// Scroll so cursor is always visible
	scroll := 0
	if a.settingsCur >= maxVisible {
		scroll = a.settingsCur - maxVisible + 1
	}
	if scroll > total-maxVisible {
		scroll = total - maxVisible
	}
	if scroll < 0 {
		scroll = 0
	}
	end := min(scroll+maxVisible, total)

	var b strings.Builder
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	for vi := scroll; vi < end; vi++ {
		realIdx := filtered[vi]
		opt := settingsOptions[realIdx]
		var toggle string
		switch realIdx {
		case 2:
			if !a.loopTrack {
				toggle = settingsOffStyle.Render("[OFF]")
			} else if a.loopTotal == 0 {
				toggle = settingsOnStyle.Render("[∞]  ")
			} else {
				toggle = settingsOnStyle.Render(fmt.Sprintf("[%dx] ", a.loopTotal))
			}
		case 12:
			toggle = actionStyle.Render("[>>>]")
		case 13:
			if a.cookieBrowser == "" {
				toggle = settingsOffStyle.Render("[OFF]")
			} else {
				toggle = settingsOnStyle.Render(fmt.Sprintf("[%-9s]", a.cookieBrowser))
			}
		case 14:
			toggle = actionStyle.Render("[>>>]")
		default:
			val := a.settingValue(realIdx)
			if val {
				toggle = settingsOnStyle.Render("[ON] ")
			} else {
				toggle = settingsOffStyle.Render("[OFF]")
			}
		}
		line := fmt.Sprintf("  %s  %-14s %s", toggle, opt.name, descStyle.Render(opt.desc))
		line = ansi.Truncate(line, innerW, "")
		if vi == a.settingsCur {
			line = settingsCurStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}

	// Show inputs or hints
	if a.settingsSearching {
		b.WriteString("\n  " + a.settingsFilterInp.View() + "\n")
		b.WriteString("\n  Enter = search  Esc = cancel")
	} else if a.settingsLoopInput {
		b.WriteString("\n  " + a.settingsLoopInp.View() + "\n")
		b.WriteString("\n  Enter = set count  Esc = cancel")
	} else if a.settingsImporting {
		b.WriteString("\n  " + a.settingsImportInput.View() + "\n")
		b.WriteString("\n  Enter = import  Esc = cancel")
	} else {
		hint := "  j/k  Enter/Space = toggle  / = search  h/l  Esc/S/q = close"
		if a.settingsFilter != "" {
			hint = fmt.Sprintf("  filter: %s  Esc = clear", a.settingsFilter)
		}
		b.WriteString("\n" + ansi.Truncate(hint, innerW, ""))
	}

	box := overlayBorderStyle.Width(boxW).Render(
		overlayTitleStyle.Render("Settings") + "\n\n" + b.String(),
	)
	return box
}

// updateColorEditor handles keys in the color editor sub-view.
func (a App) updateColorEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle text input for search or color value
	if a.colorEditorInput {
		switch {
		case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
			a.quit()
			return a, tea.Quit
		case key.Matches(msg, keys.Enter):
			val := strings.TrimSpace(a.colorEditorInp.Value())
			a.colorEditorInput = false
			a.colorEditorInp.Blur()
			if a.colorSearching {
				// Apply search filter
				a.colorFilter = val
				a.colorSearching = false
				a.colorEditorCur = 0
				// Reset prompt for next color edit
				a.colorEditorInp.Prompt = "> "
				a.colorEditorInp.Placeholder = "#ff5733"
			} else if val != "" && a.colorEditorCur < len(a.colorFilteredFields()) {
				idx := a.colorFilteredFields()[a.colorEditorCur]
				themeFields[idx].set(&a.theme, val)
				applyTheme(a.theme)
			}
			return a, nil
		case key.Matches(msg, keys.Escape):
			a.colorEditorInput = false
			a.colorEditorInp.Blur()
			if a.colorSearching {
				a.colorSearching = false
				a.colorEditorInp.Prompt = "> "
				a.colorEditorInp.Placeholder = "#ff5733"
				// Clear filter if input was empty
				if a.colorEditorInp.Value() == "" {
					a.colorFilter = ""
				}
			}
			return a, nil
		default:
			var cmd tea.Cmd
			a.colorEditorInp, cmd = a.colorEditorInp.Update(msg)
			return a, cmd
		}
	}

	switch {
	case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case key.Matches(msg, keys.Escape), msg.String() == "q":
		if a.colorFilter != "" {
			// Clear filter first
			a.colorFilter = ""
			a.colorEditorCur = 0
			return a, nil
		}
		a.showColorEditor = false
		return a, nil
	case key.Matches(msg, keys.Up):
		filtered := a.colorFilteredFields()
		if a.colorEditorCur > 0 {
			a.colorEditorCur--
		}
		a.colorEditorCur = min(a.colorEditorCur, len(filtered)-1)
		return a, nil
	case key.Matches(msg, keys.Down):
		filtered := a.colorFilteredFields()
		if a.colorEditorCur < len(filtered)-1 {
			a.colorEditorCur++
		}
		return a, nil
	case key.Matches(msg, keys.HalfDown):
		filtered := a.colorFilteredFields()
		a.colorEditorCur += len(filtered) / 2
		a.colorEditorCur = min(a.colorEditorCur, len(filtered)-1)
		return a, nil
	case key.Matches(msg, keys.HalfUp):
		filtered := a.colorFilteredFields()
		a.colorEditorCur = max(a.colorEditorCur-len(filtered)/2, 0)
		return a, nil
	case msg.String() == "g":
		a.colorEditorCur = 0
		return a, nil
	case msg.String() == "G":
		filtered := a.colorFilteredFields()
		a.colorEditorCur = len(filtered) - 1
		return a, nil
	case msg.String() == "/":
		a.colorEditorInput = true
		a.colorEditorInp.Prompt = "/"
		a.colorEditorInp.Placeholder = ""
		a.colorEditorInp.SetValue("")
		a.colorEditorInp.Focus()
		a.colorSearching = true
		return a, nil
	case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Space),
		msg.String() == "l", msg.String() == "right":
		filtered := a.colorFilteredFields()
		if a.colorEditorCur < len(filtered) {
			a.colorEditorInput = true
			idx := filtered[a.colorEditorCur]
			current := themeFields[idx].get(&a.theme)
			a.colorEditorInp.Prompt = "> "
			a.colorEditorInp.Placeholder = "#ff5733"
			a.colorEditorInp.SetValue(current)
			a.colorEditorInp.Focus()
		}
		return a, nil
	case msg.String() == "r":
		filtered := a.colorFilteredFields()
		if a.colorEditorCur < len(filtered) {
			idx := filtered[a.colorEditorCur]
			def := DefaultTheme()
			defVal := themeFields[idx].get(&def)
			themeFields[idx].set(&a.theme, defVal)
			applyTheme(a.theme)
		}
		return a, nil
	case msg.String() == "R":
		a.theme = DefaultTheme()
		applyTheme(a.theme)
		return a, nil
	}
	return a, nil
}

// renderColorEditor renders the color editor sub-view.
func (a App) renderColorEditor() string {
	boxW := min(max(a.width*2/3, 50), a.width-2)
	innerW := boxW - 6
	if innerW < 20 {
		innerW = 20
	}
	maxLines := a.height - 9
	if maxLines < 5 {
		maxLines = 5
	}

	var b strings.Builder
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(a.theme.Dimmed))
	filtered := a.colorFilteredFields()
	linesWritten := 0

	for i, idx := range filtered {
		if linesWritten >= maxLines {
			break
		}
		f := themeFields[idx]
		val := f.get(&a.theme)
		swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(val)).Render("██")
		line := fmt.Sprintf("  %s  %-14s %-10s %s", swatch, f.name, val, descStyle.Render(f.desc))
		line = ansi.Truncate(line, innerW, "")
		if i == a.colorEditorCur {
			line = settingsCurStyle.Render(line)
		}
		b.WriteString(line + "\n")
		linesWritten++
	}

	if a.colorEditorInput {
		b.WriteString("\n  " + a.colorEditorInp.View() + "\n")
		if a.colorSearching {
			b.WriteString("\n  Enter = search  Esc = cancel")
		} else {
			b.WriteString("\n  Enter = apply  Esc = cancel")
		}
	} else {
		hint := "  j/k  Enter/l = edit  / = search  r = reset  R = all  Esc = back"
		if a.colorFilter != "" {
			hint = fmt.Sprintf("  filter: %s  Esc = clear", a.colorFilter)
		}
		b.WriteString("\n" + ansi.Truncate(descStyle.Render(hint), innerW, ""))
	}

	box := overlayBorderStyle.Width(boxW).Render(
		overlayTitleStyle.Render("Colors") + "\n\n" + b.String(),
	)
	return box
}
