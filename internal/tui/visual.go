package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sadoaz/vimyt/internal/model"
)

func (a App) isVisualActive() bool {
	switch a.focusedPanel {
	case panelSearch:
		return a.search.visual
	case panelQueue:
		return a.queue.visual
	case panelPlaylist:
		return a.playlist.visual || a.playlist.listVisual
	case panelHistory:
		return a.history.visual
	case panelRadioHist:
		return a.radioHistVisual
	case panelArtists:
		return a.artistsVisual
	}
	return false
}

// updateVisual handles keys specific to visual mode.
// Returns (model, cmd, handled). If handled is false, fall through to normal.
func (a App) updateVisual(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	// y — yank selection to queue (single key in visual)
	case key.Matches(msg, keys.Yank):
		m := a.handleYank()
		return m, nil, true

	// d — delete selection
	case key.Matches(msg, keys.Delete):
		a.handleVisualDelete()
		return a, nil, true

	// x — cut selection (delete + clipboard)
	case key.Matches(msg, keys.Cut):
		a.handleVisualCut()
		return a, nil, true

	// p — paste from clipboard
	case key.Matches(msg, keys.Paste):
		a.handlePaste()
		return a, nil, true

	// F — toggle favorite for all selected tracks
	case key.Matches(msg, keys.Favorite):
		m, cmd := a.handleToggleFavorite()
		return m, cmd, true

	// a — add selection to playlist
	case key.Matches(msg, keys.AddToList):
		m, cmd := a.handleAddToPlaylist()
		return m, cmd, true

	// Block panel switching while in visual mode
	case key.Matches(msg, keys.Tab1),
		key.Matches(msg, keys.Tab2),
		key.Matches(msg, keys.Tab3),
		key.Matches(msg, keys.Tab4),
		key.Matches(msg, keys.Tab5),
		key.Matches(msg, keys.Tab6),
		key.Matches(msg, keys.TabNext),
		key.Matches(msg, keys.TabPrev),
		key.Matches(msg, keys.PanelDown),
		key.Matches(msg, keys.PanelUp),
		key.Matches(msg, keys.PanelLeft),
		key.Matches(msg, keys.PanelRight),
		key.Matches(msg, keys.Search),
		key.Matches(msg, keys.FocusSearch),
		key.Matches(msg, keys.Playlist):
		return a, nil, true
	}

	return a, nil, false
}

func (a *App) handleVisualDelete() {
	switch a.focusedPanel {
	case panelSearch:
		a.search.visual = false
	case panelHistory:
		a.history.saveUndo()
		a.history.deleteVisual()
	case panelRadioHist:
		a.radioHistDeleteVisual()
	case panelQueue:
		a.saveQueueUndo()
		a.queue.deleteVisual(a.qdata)
	case panelPlaylist:
		if a.playlist.listVisual {
			// Save all deleted playlists as a single undo entry
			lo, hi := a.playlist.listAnchor, a.playlist.listCur
			if lo > hi {
				lo, hi = hi, lo
			}
			pls := a.playlist.visiblePlaylists()
			entry := undoEntry{kind: undoMultiPlaylistDel, cursor: lo}
			for i := lo; i <= hi && i < len(pls); i++ {
				realIdx := i
				if a.playlist.isListFiltered() && i < len(a.playlist.filteredPlIdx) {
					realIdx = a.playlist.filteredPlIdx[i]
				}
				if realIdx >= 0 && realIdx < len(a.playlist.store.Playlists) {
					p := a.playlist.store.Playlists[realIdx]
					snap := make([]model.Track, len(p.Tracks))
					copy(snap, p.Tracks)
					entry.multiPl = append(entry.multiPl, struct {
						idx    int
						name   string
						tracks []model.Track
					}{idx: realIdx, name: p.Name, tracks: snap})
				}
			}
			a.undoStack = append(a.undoStack, entry)
			a.redoStack = nil
			a.playlist.deleteListVisual()
		} else {
			if !a.playlist.radioActive {
				a.saveUndo()
			}
			a.playlist.deleteVisual()
			if a.playlist.radioActive {
				a.syncRadioToQueue()
			}
		}
	}
}

func (a *App) handleVisualCut() {
	switch a.focusedPanel {
	case panelSearch:
		tracks := a.search.yankSelected()
		if len(tracks) > 0 {
			a.clipboard = tracks
			a.clipboardHadCurrent = false
		}
	case panelRadioHist:
		// Copy selected entries to radio clipboard, then delete
		visible, _ := a.radioHistVisible()
		lo, hi := a.radioHistAnchor, a.radioHistCur
		if lo > hi {
			lo, hi = hi, lo
		}
		a.radioHistClipboard = nil
		for i := lo; i <= hi && i < len(visible); i++ {
			a.radioHistClipboard = append(a.radioHistClipboard, visible[i])
		}
		a.radioHistDeleteVisual()
		return
	case panelHistory:
		tracks := a.history.yankSelected()
		if len(tracks) > 0 {
			a.history.saveUndo()
			a.clipboard = tracks
			a.clipboardHadCurrent = false
			a.history.deleteVisual()
		}
	case panelQueue:
		a.saveQueueUndo()
		// Check if currently playing track will be cut
		curID := ""
		if a.qdata.Current >= 0 && a.qdata.Current < len(a.qdata.Tracks) {
			curID = a.qdata.Tracks[a.qdata.Current].ID
		}
		tracks := a.queue.cutVisual(a.qdata)
		if len(tracks) > 0 {
			a.clipboard = tracks
			// Check if the playing track was among the cut tracks
			a.clipboardHadCurrent = false
			a.clipboardCurrentOff = 0
			for i, t := range tracks {
				if t.ID == curID && curID != "" {
					a.clipboardHadCurrent = true
					a.clipboardCurrentOff = i
					a.qdata.Current = -1 // no track in queue is playing
					break
				}
			}
		}
	case panelPlaylist:
		if a.playlist.listVisual {
			// Save playlists to clipboard before deleting
			lo, hi := a.playlist.listAnchor, a.playlist.listCur
			if lo > hi {
				lo, hi = hi, lo
			}
			pls := a.playlist.visiblePlaylists()
			a.plClipboard = nil
			for i := lo; i <= hi && i < len(pls); i++ {
				snap := make([]model.Track, len(pls[i].Tracks))
				copy(snap, pls[i].Tracks)
				a.plClipboard = append(a.plClipboard, struct {
					name   string
					tracks []model.Track
				}{name: pls[i].Name, tracks: snap})
			}
			a.handleVisualDelete() // this handles undo + deleteListVisual
			return
		}
		if !a.playlist.radioActive {
			a.saveUndo()
		}
		tracks := a.playlist.cutVisual()
		if len(tracks) > 0 {
			a.clipboard = tracks
			a.clipboardHadCurrent = false
		}
		if a.playlist.radioActive {
			a.syncRadioToQueue()
		}
	}
}
