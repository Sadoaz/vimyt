package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sadoaz/vimyt/internal/model"
)

func (a *App) handlePaste() {
	// Radio history has its own clipboard
	if a.focusedPanel == panelRadioHist {
		if len(a.radioHistClipboard) == 0 {
			return
		}
		a.radioHistSaveUndo()
		// Insert after cursor position (map display index to internal index)
		visible, realIdx := a.radioHistVisible()
		insertAfter := -1
		if a.radioHistCur >= 0 && a.radioHistCur < len(realIdx) {
			insertAfter = realIdx[a.radioHistCur]
		}
		// Entries are displayed in reverse, so "after cursor" in display = "before" in internal
		for i, entry := range a.radioHistClipboard {
			insertIdx := max(insertAfter-i, 0)
			insertIdx = min(insertIdx, len(a.radioHistory.Entries))
			newEntries := make([]model.RadioHistoryEntry, 0, len(a.radioHistory.Entries)+1)
			newEntries = append(newEntries, a.radioHistory.Entries[:insertIdx]...)
			newEntries = append(newEntries, entry)
			newEntries = append(newEntries, a.radioHistory.Entries[insertIdx:]...)
			a.radioHistory.Entries = newEntries
		}
		a.radioHistory.Save()
		_ = visible // suppress unused
		return
	}

	if a.focusedPanel == panelPlaylist && a.playlist.level == levelList && len(a.plClipboard) > 0 {
		// Paste playlists — create separate playlists with original names
		saveCur := a.playlist.listCur
		var createdIndices []int
		for _, pc := range a.plClipboard {
			pl, err := a.playlist.store.Create(pc.name)
			if err != nil {
				continue
			}
			_ = pl.AddTracks(pc.tracks...)
			createdIndices = append(createdIndices, len(a.playlist.store.Playlists)-1)
		}
		// Single undo entry for all created playlists
		if len(createdIndices) > 0 {
			var multiPl []struct {
				idx    int
				name   string
				tracks []model.Track
			}
			for _, idx := range createdIndices {
				p := a.playlist.store.Playlists[idx]
				snap := make([]model.Track, len(p.Tracks))
				copy(snap, p.Tracks)
				multiPl = append(multiPl, struct {
					idx    int
					name   string
					tracks []model.Track
				}{idx: idx, name: p.Name, tracks: snap})
			}
			a.undoStack = append(a.undoStack, undoEntry{
				kind:    undoPlaylistCreate,
				multiPl: multiPl,
				cursor:  saveCur,
			})
			a.redoStack = nil
		}
		a.playlist.listCur = len(a.playlist.store.Playlists) - 1
		if a.playlist.isListFiltered() {
			a.playlist.liveListFilter()
		}
		return
	}

	if len(a.clipboard) == 0 {
		return
	}
	switch a.focusedPanel {
	case panelPlaylist:
		if a.playlist.level == levelList {
			// In list view, paste creates a new playlist with clipboard tracks
			name := a.clipboardName
			if name == "" {
				name = "Pasted Playlist"
			}
			saveCur := a.playlist.listCur
			pl, err := a.playlist.store.Create(name)
			if err != nil {
				return
			}
			_ = pl.AddTracks(a.clipboard...)
			snap := make([]model.Track, len(a.clipboard))
			copy(snap, a.clipboard)
			newIdx := len(a.playlist.store.Playlists) - 1
			a.undoStack = append(a.undoStack, undoEntry{
				kind: undoPlaylistCreate,
				multiPl: []struct {
					idx    int
					name   string
					tracks []model.Track
				}{{idx: newIdx, name: name, tracks: snap}},
				cursor: saveCur,
			})
			a.redoStack = nil
			a.playlist.listCur = len(a.playlist.store.Playlists) - 1
			if a.playlist.isListFiltered() {
				a.playlist.liveListFilter()
			}
			return
		}
		if !a.playlist.radioActive {
			a.saveUndo()
		}
		a.playlist.pasteAfterCursor(a.clipboard)
		if a.playlist.radioActive {
			a.syncRadioToQueue()
		}
	case panelQueue:
		a.saveQueueUndo()
		realIdx := a.queue.realIndex(a.queue.cursor)
		a.qdata.InsertAfter(realIdx, a.clipboard...)
		// If the pasted tracks contain the currently playing track, update Current
		if a.clipboardHadCurrent && len(a.clipboard) > 0 {
			// Pasted tracks start at realIdx+1, playing track is at offset within clipboard
			a.qdata.Current = realIdx + 1 + a.clipboardCurrentOff
			a.clipboardHadCurrent = false
		}
		if a.queue.isFiltered() {
			a.queue.liveFilter(a.qdata)
		}
		a.queue.cursor += len(a.clipboard)
	case panelHistory:
		a.history.pasteAfterCursor(a.clipboard)
	}
}

func (a *App) handleNormalDelete() {
	switch a.focusedPanel {
	case panelHistory:
		if a.history.playHistory.Len() > 0 {
			a.history.saveUndo()
			a.history.deleteAtCursor()
		}
	case panelQueue:
		if a.qdata.Len() > 0 {
			a.saveQueueUndo()
			realIdx := a.queue.realIndex(a.queue.cursor)
			a.qdata.Remove(realIdx)
			if a.queue.isFiltered() {
				a.queue.liveFilter(a.qdata)
			}
			a.queue.clampCursor(a.queue.visibleLen(a.qdata))
		}
	case panelPlaylist:
		if a.playlist.radioActive {
			a.playlist.deleteAtCursor()
			a.syncRadioToQueue()
		} else if a.playlist.level == levelList {
			a.saveUndoPlaylistDel()
			a.playlist.deleteAtCursor()
		} else {
			a.saveUndo()
			a.playlist.deleteAtCursor()
		}
	}
}

func (a *App) handleNormalCut() {
	switch a.focusedPanel {
	case panelSearch:
		t := a.search.currentTrack()
		if t != nil {
			a.clipboard = []model.Track{*t}
			a.clipboardHadCurrent = false
		}
	case panelHistory:
		t := a.history.currentTrack()
		if t != nil {
			a.history.saveUndo()
			a.clipboard = []model.Track{*t}
			a.clipboardHadCurrent = false
			a.history.deleteAtCursor()
		}
	case panelQueue:
		if a.qdata.Len() > 0 {
			a.saveQueueUndo()
			realIdx := a.queue.realIndex(a.queue.cursor)
			a.clipboardHadCurrent = realIdx == a.qdata.Current
			a.clipboard = []model.Track{a.qdata.Tracks[realIdx]}
			a.qdata.Remove(realIdx)
			if a.clipboardHadCurrent {
				a.qdata.Current = -1 // no track in queue is playing
			}
			if a.queue.isFiltered() {
				a.queue.liveFilter(a.qdata)
			}
			a.queue.clampCursor(a.queue.visibleLen(a.qdata))
		}
	case panelPlaylist:
		if a.playlist.level == levelList {
			// Save playlist to clipboard before deleting
			p := a.playlist.currentPlaylist()
			if p != nil {
				snap := make([]model.Track, len(p.Tracks))
				copy(snap, p.Tracks)
				a.plClipboard = []struct {
					name   string
					tracks []model.Track
				}{{name: p.Name, tracks: snap}}
			}
			a.saveUndoPlaylistDel()
			a.playlist.deleteAtCursor()
			return
		}
		if !a.playlist.radioActive {
			a.saveUndo()
		}
		tracks := a.playlist.cutAtCursor()
		if len(tracks) > 0 {
			a.clipboard = tracks
			a.clipboardHadCurrent = false
		}
		if a.playlist.radioActive {
			a.syncRadioToQueue()
		}
	}
}

func (a App) handleYank() tea.Model {
	var tracks []model.Track
	switch a.focusedPanel {
	case panelSearch:
		tracks = a.search.yankSelected()
	case panelHistory:
		tracks = a.history.yankSelected()
		// Copy to clipboard only — don't add to queue
		if len(tracks) > 0 {
			a.clipboard = tracks
			a.plClipboard = nil
		}
		return a
	case panelQueue:
		tracks = a.queue.yankSelected(a.qdata)
		// In queue, yank copies to clipboard (tracks are already in queue)
		if len(tracks) > 0 {
			a.clipboard = tracks
			a.plClipboard = nil
		}
		return a
	case panelPlaylist:
		if a.playlist.level == levelList {
			// Yank entire playlists to plClipboard (for pasting as new playlists)
			a.plClipboard = nil
			a.clipboard = nil
			if a.playlist.listVisual {
				pls := a.playlist.visiblePlaylists()
				lo, hi := a.playlist.listAnchor, a.playlist.listCur
				if lo > hi {
					lo, hi = hi, lo
				}
				for i := lo; i <= hi && i < len(pls); i++ {
					snap := make([]model.Track, len(pls[i].Tracks))
					copy(snap, pls[i].Tracks)
					a.plClipboard = append(a.plClipboard, struct {
						name   string
						tracks []model.Track
					}{name: pls[i].Name, tracks: snap})
				}
				a.playlist.listVisual = false
			} else {
				p := a.playlist.currentPlaylist()
				if p != nil {
					snap := make([]model.Track, len(p.Tracks))
					copy(snap, p.Tracks)
					a.plClipboard = append(a.plClipboard, struct {
						name   string
						tracks []model.Track
					}{name: p.Name, tracks: snap})
				}
			}
			return a
		} else {
			tracks = a.playlist.yankSelected()
			if p := a.playlist.currentPlaylist(); p != nil {
				a.clipboardName = p.Name
			}
			// Copy to clipboard only — don't add to queue
			if len(tracks) > 0 {
				a.clipboard = tracks
				a.plClipboard = nil
			}
			return a
		}
	case panelArtists:
		if a.artistsLevel == 2 {
			if a.artistsVisual {
				lo, hi := a.artistsAnchor, a.artistsPanelCur
				if lo > hi {
					lo, hi = hi, lo
				}
				for i := lo; i <= hi && i < len(a.artistsPanelTrks); i++ {
					tracks = append(tracks, a.artistsPanelTrks[i])
				}
				a.artistsVisual = false
			} else if a.artistsPanelCur < len(a.artistsPanelTrks) {
				tracks = append(tracks, a.artistsPanelTrks[a.artistsPanelCur])
			}
			// Copy to clipboard only — don't add to queue
			if len(tracks) > 0 {
				a.clipboard = tracks
				a.plClipboard = nil
			}
			return a
		}
	}
	if len(tracks) == 0 {
		return a
	}
	// Copy yanked tracks to clipboard for pasting (clear plClipboard)
	a.clipboard = tracks
	a.plClipboard = nil
	a.clipboardHadCurrent = false
	a.saveQueueUndo()
	wasEmpty := a.player.Status().State == model.Stopped
	a.qdata.Add(tracks...)
	a.queue.cursor = a.qdata.Len() - 1
	if wasEmpty {
		a.pushPrev()
		a.qdata.Current = a.qdata.Len() - len(tracks)
		a.playTrack(&a.qdata.Tracks[a.qdata.Current], "queue")
	}
	return a
}
