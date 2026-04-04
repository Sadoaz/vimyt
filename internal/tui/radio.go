package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Sadoaz/vimyt/internal/model"
)

var (
	radioNormalStyle = lipgloss.NewStyle().Padding(0, 2)
	radioCurStyle    = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("238"))
)

func (a *App) radioHistSaveUndo() {
	snapshot := make([]model.RadioHistoryEntry, len(a.radioHistory.Entries))
	copy(snapshot, a.radioHistory.Entries)
	a.radioHistUndo = append(a.radioHistUndo, snapshot)
	a.radioHistRedo = nil // clear redo on new action
}

// radioHistVisible returns the visible (filtered) radio history entries, most-recent-first.
// Returns display entries and their real indices into radioHistory.Entries.
func (a *App) radioHistVisible() ([]model.RadioHistoryEntry, []int) {
	entries := a.radioHistory.Entries
	filter := strings.ToLower(a.radioHistFilter)
	words := strings.Fields(filter)
	var visible []model.RadioHistoryEntry
	var realIdx []int
	for i := len(entries) - 1; i >= 0; i-- {
		if len(words) > 0 {
			combined := strings.ToLower(entries[i].SeedTitle + " " + entries[i].SeedArtist)
			match := true
			for _, w := range words {
				if !strings.Contains(combined, w) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		visible = append(visible, entries[i])
		realIdx = append(realIdx, i)
	}
	return visible, realIdx
}

// updateRadioHistFilter handles key events when radio history filter input is active.
func (a App) updateRadioHistFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case key.Matches(msg, keys.Enter):
		a.radioHistFilter = a.radioHistFilterInput.Value()
		a.radioHistFiltering = false
		a.radioHistFilterInput.Blur()
		a.radioHistCur = 0
		a.radioHistScroll = 0
		return a, nil
	case key.Matches(msg, keys.Escape):
		if a.radioHistFilterInput.Value() != "" {
			a.radioHistFilterInput.SetValue("")
			a.radioHistFilter = ""
			a.radioHistCur = 0
			a.radioHistScroll = 0
		} else {
			a.radioHistFiltering = false
			a.radioHistFilterInput.Blur()
		}
		return a, nil
	default:
		var cmd tea.Cmd
		a.radioHistFilterInput, cmd = a.radioHistFilterInput.Update(msg)
		a.radioHistFilter = a.radioHistFilterInput.Value()
		a.radioHistCur = 0
		a.radioHistScroll = 0
		return a, cmd
	}
}

// radioHistRecover restores a radio mix from radio history.
func (a *App) radioHistRecover() tea.Cmd {
	visible, _ := a.radioHistVisible()
	total := len(visible)
	if total == 0 || a.radioHistCur >= total {
		return nil
	}
	entry := visible[a.radioHistCur]
	if len(entry.Tracks) == 0 {
		return a.setStatus("No tracks stored for this radio session")
	}

	a.saveQueueUndo()
	a.qdata.Clear()
	a.qdata.Add(entry.Tracks...)
	a.qdata.Current = 0
	a.shufflePlayed = nil
	a.cancelPrefetch()
	a.playTrack(&a.qdata.Tracks[0], "radio")
	a.queue.cursor = 0
	a.radioActive = true
	a.radioSeedTitle = entry.SeedTitle
	a.playlist.dismissRadio()

	a.pushJump(a.focusedPanel)
	a.prevPanel = a.focusedPanel
	a.focusedPanel = panelQueue
	return a.setStatus(fmt.Sprintf("Restored radio: %s (%d tracks)", entry.SeedTitle, len(entry.Tracks)))
}

// radioHistDeleteDD handles the dd (normal delete) for radio history panel.
func (a *App) radioHistDeleteDD() tea.Cmd {
	visible, realIdx := a.radioHistVisible()
	total := len(visible)
	if total == 0 || a.radioHistCur >= total {
		return nil
	}
	a.radioHistSaveUndo()
	entry := visible[a.radioHistCur]
	a.radioHistory.Remove(realIdx[a.radioHistCur])
	visible2, _ := a.radioHistVisible()
	if a.radioHistCur >= len(visible2) && a.radioHistCur > 0 {
		a.radioHistCur = len(visible2) - 1
	}
	return a.setStatus(fmt.Sprintf("Deleted radio: %s", entry.SeedTitle))
}

// radioHistDeleteVisual handles visual mode delete for radio history panel.
func (a *App) radioHistDeleteVisual() tea.Cmd {
	_, realIdx := a.radioHistVisible()
	a.radioHistSaveUndo()
	lo, hi := a.radioHistAnchor, a.radioHistCur
	if lo > hi {
		lo, hi = hi, lo
	}
	var toDelete []int
	for i := lo; i <= hi && i < len(realIdx); i++ {
		toDelete = append(toDelete, realIdx[i])
	}
	sort.Sort(sort.Reverse(sort.IntSlice(toDelete)))
	for _, idx := range toDelete {
		a.radioHistory.Remove(idx)
	}
	a.radioHistVisual = false
	a.radioHistCur = lo
	visible2, _ := a.radioHistVisible()
	if a.radioHistCur >= len(visible2) && a.radioHistCur > 0 {
		a.radioHistCur = len(visible2) - 1
	}
	a.radioHistory.Save()
	return a.setStatus(fmt.Sprintf("Deleted %d radio sessions", len(toDelete)))
}

// radioHistPerformUndo handles undo for radio history panel.
func (a *App) radioHistPerformUndo() tea.Cmd {
	if len(a.radioHistUndo) == 0 {
		return nil
	}
	redoSnap := make([]model.RadioHistoryEntry, len(a.radioHistory.Entries))
	copy(redoSnap, a.radioHistory.Entries)
	a.radioHistRedo = append(a.radioHistRedo, redoSnap)
	snap := a.radioHistUndo[len(a.radioHistUndo)-1]
	a.radioHistUndo = a.radioHistUndo[:len(a.radioHistUndo)-1]
	a.radioHistory.Entries = snap
	a.radioHistory.Save()
	a.radioHistVisual = false
	visible2, _ := a.radioHistVisible()
	if a.radioHistCur >= len(visible2) && a.radioHistCur > 0 {
		a.radioHistCur = len(visible2) - 1
	}
	return a.setStatus("Undo")
}

// radioHistPerformRedo handles redo for radio history panel.
func (a *App) radioHistPerformRedo() tea.Cmd {
	if len(a.radioHistRedo) == 0 {
		return nil
	}
	undoSnap := make([]model.RadioHistoryEntry, len(a.radioHistory.Entries))
	copy(undoSnap, a.radioHistory.Entries)
	a.radioHistUndo = append(a.radioHistUndo, undoSnap)
	snap := a.radioHistRedo[len(a.radioHistRedo)-1]
	a.radioHistRedo = a.radioHistRedo[:len(a.radioHistRedo)-1]
	a.radioHistory.Entries = snap
	a.radioHistory.Save()
	a.radioHistVisual = false
	visible2, _ := a.radioHistVisible()
	if a.radioHistCur >= len(visible2) && a.radioHistCur > 0 {
		a.radioHistCur = len(visible2) - 1
	}
	return a.setStatus("Redo")
}

// renderRadioHistPanel renders radio history entries for the panel (full view).
func (a App) renderRadioHistConstrained(width, height int) string {
	visible, _ := a.radioHistVisible()
	focused := a.focusedPanel == panelRadioHist

	var b strings.Builder

	if len(visible) == 0 {
		if a.radioHistFilter != "" {
			b.WriteString("  No matching radio sessions.")
		} else {
			b.WriteString("  No radio sessions yet.")
		}
		return b.String()
	}

	if a.radioHistFilter != "" {
		filterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
		b.WriteString(filterStyle.Render(fmt.Sprintf("  (filter: %s)", a.radioHistFilter)))
		b.WriteString("\n\n")
	}

	maxVisible := height
	if a.radioHistFilter != "" {
		maxVisible -= 2 // filter label + blank line
	}
	if maxVisible < 1 {
		maxVisible = 10
	}

	// Adjust scroll to keep cursor visible
	if a.radioHistCur < a.radioHistScroll {
		a.radioHistScroll = a.radioHistCur
	}
	if a.radioHistCur >= a.radioHistScroll+maxVisible {
		a.radioHistScroll = a.radioHistCur - maxVisible + 1
	}
	a.radioHistScroll = min(a.radioHistScroll, len(visible)-maxVisible)
	a.radioHistScroll = max(a.radioHistScroll, 0)
	start := a.radioHistScroll
	end := min(start+maxVisible, len(visible))

	for di := start; di < end; di++ {
		e := visible[di]
		lineNum := di + 1
		isCursor := di == a.radioHistCur && focused
		isSel := false
		if a.radioHistVisual && focused {
			lo, hi := a.radioHistAnchor, a.radioHistCur
			if lo > hi {
				lo, hi = hi, lo
			}
			isSel = di >= lo && di <= hi
		}
		style := radioNormalStyle
		prefix := "  "
		if isCursor {
			style = radioCurStyle
			prefix = "> "
		} else if isSel {
			style = radioCurStyle
		}
		var line string
		hasBackground := isCursor || isSel
		if hasBackground {
			plain := fmt.Sprintf("%2d  %s — %s  (%d tracks)",
				lineNum, e.SeedTitle, e.SeedArtist, e.TrackCount)
			content := prefix + plain
			rendered := style.Render(content)
			if isCursor && width > 0 {
				line = marquee(rendered, width, a.tickCount)
			} else if width > 0 {
				line = ansi.Truncate(rendered, width, "")
			} else {
				line = rendered
			}
		} else {
			trackInfo := plCountStyle.Render(fmt.Sprintf("(%d tracks)", e.TrackCount))
			plain := fmt.Sprintf("%2d  %s — %s  %s",
				lineNum,
				e.SeedTitle,
				artistStyle.Render(e.SeedArtist),
				trackInfo,
			)
			content := prefix + plain
			rendered := style.Render(content)
			if width > 0 {
				line = ansi.Truncate(rendered, width, "…")
			} else {
				line = rendered
			}
		}
		b.WriteString(line + "\n")
	}

	return b.String()
}

// renderRadioHistCompact renders a compact summary for radio history when not focused.
func (a App) renderRadioHistCompact(width int) string {
	count := len(a.radioHistory.Entries)
	line := fmt.Sprintf("  %d radio sessions", count)
	if width > 0 {
		line = ansi.Truncate(line, width, "")
	}
	return line + "\n"
}
