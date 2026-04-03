package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (a App) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ensure search input is blurred when not in search panel
	if a.focusedPanel != panelSearch && a.search.input.Focused() {
		a.search.blur()
	}

	// Handle GOTO input mode
	if a.gotoActive {
		return a.updateGotoInput(msg)
	}

	// Handle yy sequence
	if a.waitingY {
		a.waitingY = false
		if msg.String() == "y" {
			m := a.handleYank()
			return m, nil
		}
		return a, nil
	}

	// Handle dd sequence
	if a.waitingD {
		a.waitingD = false
		if msg.String() == "d" {
			a.handleNormalDelete()
			return a, nil
		}
		return a, nil
	}

	// Handle dd sequence for radio history panel
	if a.radioHistWaitD {
		a.radioHistWaitD = false
		if msg.String() == "d" {
			cmd := a.radioHistDeleteDD()
			return a, cmd
		}
		return a, nil
	}

	switch {
	// Quit
	case key.Matches(msg, keys.Quit):
		a.quit()
		return a, tea.Quit

	// Help
	case key.Matches(msg, keys.Help):
		a.showHelp = true
		a.helpScroll = 0
		a.helpFilter = ""
		a.helpFiltering = false
		return a, nil

	// View switching
	case key.Matches(msg, keys.Tab1):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelSearch
		return a, nil
	case key.Matches(msg, keys.Tab2), key.Matches(msg, keys.Playlist):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelPlaylist
		return a, nil
	case key.Matches(msg, keys.Tab3):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelQueue
		return a, nil
	case key.Matches(msg, keys.Tab4):
		if a.showHistory {
			a.pushJump(a.focusedPanel)
			a.prevPanel = a.focusedPanel
			a.focusedPanel = panelHistory
		}
		return a, nil
	case key.Matches(msg, keys.TabNext):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelSearch:
			a.focusedPanel = panelPlaylist
		case panelPlaylist:
			if a.showHistory {
				a.focusedPanel = panelHistory
			} else if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelQueue
			}
		case panelHistory:
			if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelQueue
			}
		case panelRadioHist:
			a.focusedPanel = panelQueue
		case panelQueue:
			a.focusedPanel = panelSearch
		}
		return a, nil
	case key.Matches(msg, keys.TabPrev):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelSearch:
			a.focusedPanel = panelQueue
		case panelQueue:
			if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else if a.showHistory {
				a.focusedPanel = panelHistory
			} else {
				a.focusedPanel = panelPlaylist
			}
		case panelRadioHist:
			if a.showHistory {
				a.focusedPanel = panelHistory
			} else {
				a.focusedPanel = panelPlaylist
			}
		case panelHistory:
			a.focusedPanel = panelPlaylist
		case panelPlaylist:
			a.focusedPanel = panelSearch
		}
		return a, nil

	// / = always jump to search panel and focus input
	case key.Matches(msg, keys.Search):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelSearch
		cmd := a.search.focus()
		return a, cmd

	// f = filter in playlist/queue/search
	case key.Matches(msg, keys.Filter):
		switch a.focusedPanel {
		case panelSearch:
			cmd := a.search.startFilter()
			return a, cmd
		case panelPlaylist:
			cmd := a.playlist.startFilter()
			return a, cmd
		case panelQueue:
			cmd := a.queue.startFilter()
			return a, cmd
		case panelHistory:
			cmd := a.history.startFilter()
			return a, cmd
		case panelRadioHist:
			a.radioHistFiltering = true
			a.radioHistFilterInput.SetValue(a.radioHistFilter)
			a.radioHistFilterInput.Placeholder = "Filter radio history..."
			a.radioHistFilterInput.Focus()
			return a, nil
		}
		return a, nil

	// Navigation
	case key.Matches(msg, keys.Up):
		switch a.focusedPanel {
		case panelSearch:
			a.search.moveUp()
		case panelQueue:
			a.queue.moveUp()
		case panelPlaylist:
			a.playlist.moveUp()
		case panelHistory:
			a.history.moveUp()
		case panelRadioHist:
			if a.radioHistCur > 0 {
				a.radioHistCur--
			}
		}
		return a, nil

	case key.Matches(msg, keys.Down):
		switch a.focusedPanel {
		case panelSearch:
			a.search.moveDown()
		case panelQueue:
			a.queue.moveDown(a.queue.visibleLen(a.qdata))
		case panelPlaylist:
			a.playlist.moveDown()
		case panelHistory:
			a.history.moveDown()
		case panelRadioHist:
			visible, _ := a.radioHistVisible()
			if a.radioHistCur < len(visible)-1 {
				a.radioHistCur++
			}
		}
		return a, nil

	// Half-page scroll (motions, not jumps — don't push to jumplist)
	case key.Matches(msg, keys.HalfDown):
		halfPage := max(a.height/4, 2) // quarter of terminal = half of a typical panel
		switch a.focusedPanel {
		case panelSearch:
			a.search.halfPageDown(halfPage * 2)
		case panelQueue:
			a.queue.halfPageDown(a.queue.visibleLen(a.qdata), halfPage*2)
		case panelPlaylist:
			a.playlist.halfPageDown(halfPage * 2)
		case panelHistory:
			a.history.halfPageDown(halfPage * 2)
		case panelRadioHist:
			visible, _ := a.radioHistVisible()
			a.radioHistCur += halfPage
			a.radioHistCur = min(a.radioHistCur, len(visible)-1)
			a.radioHistCur = max(a.radioHistCur, 0)
		}
		return a, nil

	case key.Matches(msg, keys.HalfUp):
		halfPage := max(a.height/4, 2)
		switch a.focusedPanel {
		case panelSearch:
			a.search.halfPageUp(halfPage * 2)
		case panelQueue:
			a.queue.halfPageUp(halfPage * 2)
		case panelPlaylist:
			a.playlist.halfPageUp(halfPage * 2)
		case panelHistory:
			a.history.halfPageUp(halfPage * 2)
		case panelRadioHist:
			a.radioHistCur -= halfPage
			a.radioHistCur = max(a.radioHistCur, 0)
		}
		return a, nil

	case key.Matches(msg, keys.Top):
		a.gotoActive = true
		a.gotoInput.SetValue("")
		a.gotoInput.Focus()
		return a, nil

	case key.Matches(msg, keys.Bottom):

		switch a.focusedPanel {
		case panelSearch:
			a.search.goBottom()
		case panelQueue:
			a.queue.goBottom(a.queue.visibleLen(a.qdata))
		case panelPlaylist:
			a.playlist.goBottom()
		case panelHistory:
			a.history.goBottom()
		case panelRadioHist:
			visible, _ := a.radioHistVisible()
			if len(visible) > 0 {
				a.radioHistCur = len(visible) - 1
			}
		}
		return a, nil

	// Visual select
	case key.Matches(msg, keys.Visual):
		switch a.focusedPanel {
		case panelSearch:
			a.search.toggleVisual()
		case panelQueue:
			a.queue.toggleVisual()
		case panelPlaylist:
			a.playlist.toggleVisual()
		case panelHistory:
			a.history.toggleVisual()
		case panelRadioHist:
			a.radioHistVisual = !a.radioHistVisual
			if a.radioHistVisual {
				a.radioHistAnchor = a.radioHistCur
			}
		}
		return a, nil

	// Swap visual selection end / Create playlist with 'o'
	case key.Matches(msg, keys.VisualEnd):
		// 'o' in playlist list view (not visual) = create new playlist
		if a.focusedPanel == panelPlaylist && a.playlist.level == levelList && !a.playlist.visual && !a.playlist.listVisual {
			cmd := a.playlist.startCreate()
			return a, cmd
		}
		switch a.focusedPanel {
		case panelSearch:
			a.search.swapVisualEnd()
		case panelQueue:
			a.queue.swapVisualEnd()
		case panelPlaylist:
			a.playlist.swapVisualEnd()
		case panelHistory:
			a.history.swapVisualEnd()
		case panelRadioHist:
			if a.radioHistVisual {
				a.radioHistAnchor, a.radioHistCur = a.radioHistCur, a.radioHistAnchor
			}
		}
		return a, nil

	// Yank (first 'y')
	case key.Matches(msg, keys.Yank):
		a.waitingY = true
		return a, nil

	// Enter
	case key.Matches(msg, keys.Enter):
		return a.handleEnter()

	// Playback controls
	case key.Matches(msg, keys.Space):
		a.player.Pause()
		return a, nil

	case key.Matches(msg, keys.Next):
		if a.qdata.Len() == 0 {
			return a, nil
		}
		a.pushPrev()
		if a.shuffle {
			idx := a.pickShuffleNext()
			a.qdata.Current = idx
			a.playTrack(&a.qdata.Tracks[idx], "queue")
		} else {
			t := a.qdata.Next()
			if t != nil {
				a.playTrack(t, "queue")
			} else {
				a.player.Stop()
			}
		}
		a.cancelPrefetch() // kill in-flight prefetch after manual skip
		return a, nil

	case key.Matches(msg, keys.Prev):
		if len(a.prevStack) == 0 {
			return a, nil
		}
		idx := a.prevStack[len(a.prevStack)-1]
		a.prevStack = a.prevStack[:len(a.prevStack)-1]
		if idx >= 0 && idx < a.qdata.Len() {
			a.qdata.Current = idx
			a.player.Play(&a.qdata.Tracks[idx])
			if a.playHistory != nil {
				a.playHistory.Add(a.qdata.Tracks[idx], "queue")
			}
		}
		return a, nil

	// >/< = seek
	case key.Matches(msg, keys.SeekFwd):
		a.player.Seek(5)
		return a, nil

	case key.Matches(msg, keys.SeekBack):
		a.player.Seek(-5)
		return a, nil

	// +/- = volume
	case key.Matches(msg, keys.VolumeUp):
		status := a.player.Status()
		newVol := min(status.Volume+5, 100)
		a.player.SetVolume(newVol)
		cmd := a.setStatus(fmt.Sprintf("Volume: %d%%", newVol))
		return a, cmd

	case key.Matches(msg, keys.VolumeDown):
		status := a.player.Status()
		newVol := max(status.Volume-5, 0)
		a.player.SetVolume(newVol)
		cmd := a.setStatus(fmt.Sprintf("Volume: %d%%", newVol))
		return a, cmd

	// Shift+HJKL = panel navigation
	// Layout: Search (top full width)
	//         Playlists (left top)  | Queue (right)
	//         History (left bottom) |
	case key.Matches(msg, keys.PanelDown):
		a.pushJump(a.focusedPanel)
		prev := a.prevPanel
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelSearch:
			// Return to the panel we came from, default to playlist
			if prev == panelQueue || prev == panelHistory || prev == panelRadioHist {
				if prev == panelRadioHist && !a.showRadio {
					a.focusedPanel = panelPlaylist
				} else if prev == panelHistory && !a.showHistory {
					a.focusedPanel = panelPlaylist
				} else {
					a.focusedPanel = prev
				}
			} else {
				a.focusedPanel = panelPlaylist
			}
		case panelPlaylist:
			if a.showHistory {
				a.focusedPanel = panelHistory
			} else if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelSearch
			}
		case panelHistory:
			if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelSearch
			}
		case panelRadioHist:
			a.focusedPanel = panelSearch
		case panelQueue:
			a.focusedPanel = panelSearch
		}
		return a, nil
	case key.Matches(msg, keys.PanelUp):
		a.pushJump(a.focusedPanel)
		prev := a.prevPanel
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelSearch:
			// Return to the panel we came from, default to bottom panels
			if prev == panelQueue || prev == panelPlaylist || prev == panelHistory || prev == panelRadioHist {
				if prev == panelRadioHist && !a.showRadio {
					a.focusedPanel = panelPlaylist
				} else if prev == panelHistory && !a.showHistory {
					a.focusedPanel = panelPlaylist
				} else {
					a.focusedPanel = prev
				}
			} else if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else if a.showHistory {
				a.focusedPanel = panelHistory
			} else {
				a.focusedPanel = panelPlaylist
			}
		case panelPlaylist:
			a.focusedPanel = panelSearch
		case panelHistory:
			a.focusedPanel = panelPlaylist
		case panelRadioHist:
			if a.showHistory {
				a.focusedPanel = panelHistory
			} else {
				a.focusedPanel = panelPlaylist
			}
		case panelQueue:
			a.focusedPanel = panelSearch
		}
		return a, nil
	case key.Matches(msg, keys.PanelRight):
		a.pushJump(a.focusedPanel)
		prev := a.prevPanel
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelPlaylist:
			a.focusedPanel = panelQueue
		case panelHistory:
			a.focusedPanel = panelQueue
		case panelRadioHist:
			a.focusedPanel = panelQueue
		case panelQueue:
			// Return to left panel we came from
			if prev == panelHistory && a.showHistory {
				a.focusedPanel = panelHistory
			} else if prev == panelRadioHist && a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelPlaylist
			}
		case panelSearch:
			a.focusedPanel = panelQueue
		}
		return a, nil
	case key.Matches(msg, keys.PanelLeft):
		a.pushJump(a.focusedPanel)
		prev := a.prevPanel
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelQueue:
			// Return to left panel we came from
			if prev == panelHistory && a.showHistory {
				a.focusedPanel = panelHistory
			} else if prev == panelRadioHist && a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelPlaylist
			}
		case panelPlaylist:
			a.focusedPanel = panelQueue
		case panelHistory:
			a.focusedPanel = panelQueue
		case panelRadioHist:
			a.focusedPanel = panelQueue
		case panelSearch:
			a.focusedPanel = panelPlaylist
		}
		return a, nil

	// Ctrl+S = focus search input from anywhere
	case key.Matches(msg, keys.FocusSearch):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelSearch
		cmd := a.search.focus()
		return a, cmd

	// z = toggle zoom on focused panel
	case key.Matches(msg, keys.Zoom):
		a.zoomed = !a.zoomed
		return a, nil

	// backspace = back
	case key.Matches(msg, keys.Backspace):
		return a.handleNavBack()
	case key.Matches(msg, keys.NavBack):
		return a.handleNavBack()

	// l = navigate forward (enter a playlist, etc.) — does NOT play tracks
	case key.Matches(msg, keys.NavForward):
		return a.handleNavForward()

	// Escape — same as h/backspace (go back)
	case key.Matches(msg, keys.Escape):
		return a.handleNavBack()

	// Add to playlist (a) — from any view with tracks
	case key.Matches(msg, keys.AddToList):
		return a.handleAddToPlaylist()

	// Create playlist (c) — in playlist list view; jump to current track in queue
	case key.Matches(msg, keys.CreatePL):
		// 'c' always goes to currently playing track in queue
		if a.qdata.Current >= 0 && a.qdata.Current < a.qdata.Len() {
			if a.focusedPanel != panelQueue {
				a.pushJump(a.focusedPanel)
				a.prevPanel = a.focusedPanel
				a.focusedPanel = panelQueue
			}
			if a.queue.isFiltered() {
				for fi, ri := range a.queue.filteredIdx {
					if ri == a.qdata.Current {
						a.queue.cursor = fi
						break
					}
				}
			} else {
				a.queue.cursor = a.qdata.Current
			}
			a.queue.ensureVisible()
		}
		return a, nil

	// Edit/rename playlist (e) — in playlist list view
	case key.Matches(msg, keys.EditPL):
		if a.focusedPanel == panelPlaylist && a.playlist.level == levelList {
			cmd := a.playlist.startRename()
			return a, cmd
		}
		return a, nil

	// Radio (r) — start radio from any track
	case key.Matches(msg, keys.Radio):
		return a.handleRadio()

	// Toggle favorite (F) — add/remove track from Favorites playlist
	case key.Matches(msg, keys.Favorite):
		return a.handleToggleFavorite()

	// Settings (S)
	case key.Matches(msg, keys.Settings):
		a.showSettings = true
		a.settingsCur = 0
		return a, nil

	// Radio history (5)
	case key.Matches(msg, keys.Tab5):
		if a.showRadio {
			a.pushJump(a.focusedPanel)
			a.prevPanel = a.focusedPanel
			a.focusedPanel = panelRadioHist
		}
		return a, nil

	// Delete (first 'd' of dd)
	case key.Matches(msg, keys.Delete):
		if a.focusedPanel == panelRadioHist {
			if a.radioHistVisual {
				cmd := a.radioHistDeleteVisual()
				return a, cmd
			}
			a.radioHistWaitD = true
			return a, nil
		}
		a.waitingD = true
		return a, nil

	// Cut
	case key.Matches(msg, keys.Cut):
		a.handleNormalCut()
		return a, nil

	// Paste
	case key.Matches(msg, keys.Paste):
		a.handlePaste()
		return a, nil

	// J/K = panel navigation (vertical)
	case key.Matches(msg, keys.MoveDown):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelSearch:
			a.focusedPanel = panelPlaylist
		case panelPlaylist:
			a.focusedPanel = panelQueue
		case panelQueue:
			if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelSearch
			}
		case panelRadioHist:
			a.focusedPanel = panelSearch
		}
		return a, nil

	case key.Matches(msg, keys.MoveUp):
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		switch a.focusedPanel {
		case panelSearch:
			if a.showRadio {
				a.focusedPanel = panelRadioHist
			} else {
				a.focusedPanel = panelQueue
			}
		case panelPlaylist:
			a.focusedPanel = panelSearch
		case panelQueue:
			a.focusedPanel = panelPlaylist
		case panelRadioHist:
			a.focusedPanel = panelQueue
		}
		return a, nil

	case key.Matches(msg, keys.Clear):
		if a.focusedPanel == panelQueue {

			a.saveQueueUndo()
			a.qdata.Clear()
			a.player.Stop()
			a.queue.cursor = 0
			a.queue.visual = false
			a.queue.clearFilter()
			a.radioActive = false
			a.shufflePlayed = nil
			a.cancelPrefetch()
		}
		return a, nil

	// Randomize queue order (R)
	case key.Matches(msg, keys.Randomize):
		if a.focusedPanel == panelQueue && a.qdata.Len() > 1 {
			a.saveQueueUndo()
			a.qdata.Shuffle()
			a.queue.cursor = 0
			a.queue.scroll = 0
			a.queue.visual = false
			a.queue.clearFilter()
			a.shufflePlayed = nil
			cmd := a.setStatus("Queue randomized")
			return a, cmd
		}
		return a, nil

	// Undo
	case key.Matches(msg, keys.Undo):
		if a.focusedPanel == panelRadioHist {
			cmd := a.radioHistPerformUndo()
			if cmd != nil {
				return a, cmd
			}
		}
		if a.focusedPanel == panelHistory {
			if a.history.performUndo() {
				cmd := a.setStatus("Undo history")
				return a, cmd
			}
		}
		// Find the most recent undo entry matching the current panel;
		// also handles cross-panel undo (e.g. undoFavorite from any panel).
		statusMsg := a.performUndoForPanel(a.focusedPanel)
		cmd := a.setStatus(statusMsg)
		return a, cmd

	// Redo
	case key.Matches(msg, keys.Redo):
		if a.focusedPanel == panelRadioHist {
			cmd := a.radioHistPerformRedo()
			if cmd != nil {
				return a, cmd
			}
		}
		if a.focusedPanel == panelHistory {
			if a.history.performRedo() {
				cmd := a.setStatus("Redo history")
				return a, cmd
			}
		}
		statusMsg := a.performRedoForPanel(a.focusedPanel)
		cmd := a.setStatus(statusMsg)
		return a, cmd

	// Jumplist navigation
	case key.Matches(msg, keys.JumpBack):
		if len(a.jumpBack) > 0 {
			target := a.jumpBack[len(a.jumpBack)-1]
			a.jumpBack = a.jumpBack[:len(a.jumpBack)-1]
			a.jumpFwd = append(a.jumpFwd, a.focusedPanel)
			if len(a.jumpFwd) > 50 {
				a.jumpFwd = a.jumpFwd[len(a.jumpFwd)-50:]
			}
			a.focusedPanel = target
		}
		return a, nil
	case key.Matches(msg, keys.JumpFwd):
		if len(a.jumpFwd) > 0 {
			target := a.jumpFwd[len(a.jumpFwd)-1]
			a.jumpFwd = a.jumpFwd[:len(a.jumpFwd)-1]
			a.jumpBack = append(a.jumpBack, a.focusedPanel)
			if len(a.jumpBack) > 50 {
				a.jumpBack = a.jumpBack[len(a.jumpBack)-50:]
			}
			a.focusedPanel = target
		}
		return a, nil

	// Colon command
	case key.Matches(msg, keys.Colon):
		a.colonActive = true
		a.colonInput.SetValue("")
		cmd := a.colonInput.Focus()
		return a, cmd
	}

	return a, nil
}

func (a App) updateGotoInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case key.Matches(msg, keys.Enter):
		val := strings.TrimSpace(a.gotoInput.Value())
		a.gotoActive = false
		a.gotoInput.Blur()
		if val == "" {
			return a, nil
		}
		secs, ok := parseGotoTime(val)
		if !ok {
			cmd := a.setStatus("Invalid time format (use 30, 1:23, or 1:23:45)")
			return a, cmd
		}
		a.player.SeekAbsolute(float64(secs))
		h := secs / 3600
		m := (secs % 3600) / 60
		s := secs % 60
		var timeStr string
		if h > 0 {
			timeStr = fmt.Sprintf("%d:%02d:%02d", h, m, s)
		} else {
			timeStr = fmt.Sprintf("%d:%02d", m, s)
		}
		cmd := a.setStatus(fmt.Sprintf("Seek to %s", timeStr))
		return a, cmd
	case key.Matches(msg, keys.Escape):
		a.gotoActive = false
		a.gotoInput.Blur()
		return a, nil
	case msg.String() == "g":
		// 'g' in GOTO input with empty value = gg (go to top)
		if a.gotoInput.Value() == "" {
			a.gotoActive = false
			a.gotoInput.Blur()

			switch a.focusedPanel {
			case panelSearch:
				a.search.goTop()
			case panelQueue:
				a.queue.goTop()
			case panelPlaylist:
				a.playlist.goTop()
			case panelHistory:
				a.history.goTop()
			case panelRadioHist:
				a.radioHistCur = 0
				a.radioHistScroll = 0
			}
			return a, nil
		}
		// Otherwise pass 'g' through to the input (shouldn't normally happen)
		fallthrough
	default:
		var cmd tea.Cmd
		a.gotoInput, cmd = a.gotoInput.Update(msg)
		return a, cmd
	}
}

// parseGotoTime parses time strings: "30" (seconds), "1:23" (min:sec), "1:23:45" (hr:min:sec).
func parseGotoTime(s string) (int, bool) {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 1:
		n, err := strconv.Atoi(parts[0])
		if err != nil || n < 0 {
			return 0, false
		}
		return n, true
	case 2:
		m, err1 := strconv.Atoi(parts[0])
		sec, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || m < 0 || sec < 0 || sec >= 60 {
			return 0, false
		}
		return m*60 + sec, true
	case 3:
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		sec, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil || h < 0 || m < 0 || m >= 60 || sec < 0 || sec >= 60 {
			return 0, false
		}
		return h*3600 + m*60 + sec, true
	default:
		return 0, false
	}
}

func (a App) updateColonInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case key.Matches(msg, keys.Enter):
		val := strings.TrimSpace(a.colonInput.Value())
		// :history — focus radio history panel
		if val == "history" {
			a.colonActive = false
			a.colonInput.Blur()
			a.pushJump(a.focusedPanel)
			a.prevPanel = a.focusedPanel
			a.focusedPanel = panelRadioHist
			return a, nil
		}
		if n, err := strconv.Atoi(val); err == nil && n > 0 {

			// Jump to absolute line number (1-indexed)
			target := n - 1
			switch a.focusedPanel {
			case panelSearch:
				visible := a.search.visibleTracks()
				a.search.cursor = min(target, len(visible)-1)
			case panelQueue:
				qLen := a.queue.visibleLen(a.qdata)
				a.queue.cursor = min(target, qLen-1)
			case panelPlaylist:
				if a.playlist.level == levelList {
					pls := a.playlist.visiblePlaylists()
					a.playlist.listCur = min(target, len(pls)-1)
				} else {
					tracks := a.playlist.visibleTracks()
					a.playlist.detailCur = min(target, len(tracks)-1)
				}
			case panelHistory:
				tracks := a.history.tracks()
				a.history.cursor = min(target, len(tracks)-1)
			case panelRadioHist:
				visible, _ := a.radioHistVisible()
				a.radioHistCur = min(target, len(visible)-1)
			}
		}
		a.colonActive = false
		a.colonInput.Blur()
		return a, nil
	case key.Matches(msg, keys.Escape):
		a.colonActive = false
		a.colonInput.Blur()
		return a, nil
	default:
		var cmd tea.Cmd
		a.colonInput, cmd = a.colonInput.Update(msg)
		return a, cmd
	}
}

func (a App) updateSearchInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	kmsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		a.search, cmd = a.search.Update(msg)
		return a, cmd
	}

	switch {
	case key.Matches(kmsg, keys.Quit) && kmsg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case key.Matches(kmsg, keys.Enter):
		cmd := a.search.submit()
		return a, cmd
	case key.Matches(kmsg, keys.Escape):
		a.search.blur()
		return a, nil
	// ctrl+j / ctrl+k — navigate results while input stays focused
	case kmsg.String() == "ctrl+j":
		a.search.moveDown()
		return a, nil
	case kmsg.String() == "ctrl+k":
		a.search.moveUp()
		return a, nil
	// Panel switching from insert mode
	case kmsg.String() == "ctrl+p":
		a.search.blur()
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelPlaylist
		return a, nil
	case kmsg.String() == "ctrl+q":
		a.search.blur()
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelQueue
		return a, nil
	default:
		var cmd tea.Cmd
		a.search, cmd = a.search.Update(msg)
		return a, cmd
	}
}
