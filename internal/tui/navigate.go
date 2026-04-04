package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sadoaz/vimyt/internal/model"
	"github.com/Sadoaz/vimyt/internal/youtube"
)

func (a App) handleEnter() (tea.Model, tea.Cmd) {
	// If visual mode is active, add all selected tracks to queue and play the first
	if a.isVisualActive() {
		// For radio history, Enter in visual mode just recovers the current entry
		if a.focusedPanel == panelRadioHist {
			a.radioHistVisual = false
			cmd := a.radioHistRecover()
			return a, cmd
		}
		var tracks []model.Track
		switch a.focusedPanel {
		case panelSearch:
			tracks = a.search.yankSelected()
		case panelHistory:
			tracks = a.history.yankSelected()
		case panelQueue:
			tracks = a.queue.yankSelected(a.qdata)
		case panelPlaylist:
			tracks = a.playlist.yankSelected()
		}
		if len(tracks) > 0 {
			a.saveQueueUndo()
			for _, t := range tracks {
				a.qdata.Add(t)
			}
			a.queue.cursor = a.qdata.Len() - 1
			if a.autoFocusQueue && a.focusedPanel != panelQueue {
				a.pushJump(a.focusedPanel)
				a.prevPanel = a.focusedPanel
				a.focusedPanel = panelQueue
			}
			cmd := a.setStatus(fmt.Sprintf("Added %d tracks to queue", len(tracks)))
			return a, cmd
		}
		return a, nil
	}

	played := false
	switch a.focusedPanel {
	case panelSearch:
		t := a.search.currentTrack()
		if t == nil {
			return a, nil
		}
		a.pushPrev()
		a.saveQueueUndo()
		a.qdata.Add(*t)
		a.queue.cursor = a.qdata.Len() - 1
		a.qdata.Current = a.qdata.Len() - 1
		a.playTrack(t, "search")
		played = true
	case panelQueue:
		// Enter in queue plays the track under cursor
		realIdx := a.queue.realIndex(a.queue.cursor)
		if a.qdata.Len() > 0 && realIdx < a.qdata.Len() {
			a.pushPrev()
			a.qdata.Current = realIdx
			a.playTrack(&a.qdata.Tracks[a.qdata.Current], "queue")
		}
	case panelHistory:
		t := a.history.currentTrack()
		if t == nil {
			return a, nil
		}
		a.pushPrev()
		a.saveQueueUndo()
		a.qdata.Add(*t)
		a.queue.cursor = a.qdata.Len() - 1
		a.qdata.Current = a.qdata.Len() - 1
		a.playTrack(t, "history")
		played = true
	case panelRadioHist:
		cmd := a.radioHistRecover()
		// Radio recover already focuses queue
		return a, cmd
	case panelArtists:
		cmd := a.artistsPanelEnter()
		return a, cmd
	case panelPlaylist:
		if a.playlist.level == levelDetail {
			// In detail: Enter plays the track under cursor
			tracks := a.playlist.visibleTracks()
			if a.playlist.detailCur < len(tracks) {
				t := tracks[a.playlist.detailCur]
				a.pushPrev()
				a.saveQueueUndo()
				a.qdata.Add(t)
				a.queue.cursor = a.qdata.Len() - 1
				a.qdata.Current = a.qdata.Len() - 1
				a.playTrack(&a.qdata.Tracks[a.qdata.Current], "playlist")
				played = true
			}
		} else {
			a.playlist.enter()
		}
	}
	// Auto-focus queue after playing a track (if setting is enabled)
	if played && a.autoFocusQueue && a.focusedPanel != panelQueue {
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelQueue
	}
	return a, nil
}

// handleNavForward implements l = navigate forward (enter a playlist, etc.)
// Unlike handleEnter, this does NOT play tracks — it only navigates into sub-levels.
func (a App) handleNavForward() (tea.Model, tea.Cmd) {
	switch a.focusedPanel {
	case panelPlaylist:
		if a.playlist.level == levelList {

			a.playlist.enter()
		}
		// In detail view: l does nothing (Enter plays tracks)
	case panelArtists:
		cmd := a.artistsPanelEnter()
		return a, cmd
	}
	// Search and Queue have no sub-levels to navigate into
	return a, nil
}

// handleNavBack implements h/backspace = go back one level within a panel.
func (a App) handleNavBack() (tea.Model, tea.Cmd) {
	switch a.focusedPanel {
	case panelPlaylist:
		if a.playlist.visual {
			a.playlist.visual = false
		} else if a.playlist.listVisual {
			a.playlist.listVisual = false
		} else {
			// back() returns true if at top level with nothing to do
			a.playlist.back()
			// NOTE: when leaving radio view, we intentionally keep app-level
			// radioActive because the queue still has radio tracks.
		}
	case panelSearch:
		if a.search.visual {
			a.search.visual = false
		} else if a.search.isFiltered() {
			a.search.clearFilter()
			a.search.cursor = 0
		}
		// search is top-level, nothing else to go back to
	case panelQueue:
		if a.queue.visual {
			a.queue.visual = false
		} else if a.queue.isFiltered() {
			a.queue.clearFilter()
			a.queue.cursor = 0
		}
		// queue is top-level, nothing else to go back to
	case panelHistory:
		if a.history.visual {
			a.history.visual = false
		} else if a.history.isFiltered() {
			a.history.clearFilter()
			a.history.cursor = 0
		}
		// history is top-level, nothing else to go back to
	case panelRadioHist:
		if a.radioHistVisual {
			a.radioHistVisual = false
		} else if a.radioHistFilter != "" {
			a.radioHistFilter = ""
			a.radioHistFilterInput.SetValue("")
			a.radioHistCur = 0
			a.radioHistScroll = 0
		}
	case panelArtists:
		a.artistsPanelBack()
	}
	return a, nil
}

// handleRadio starts a radio mix from the track under the cursor.
func (a App) handleRadio() (tea.Model, tea.Cmd) {
	var seed *model.Track

	switch a.focusedPanel {
	case panelSearch:
		seed = a.search.currentTrack()
	case panelQueue:
		realIdx := a.queue.realIndex(a.queue.cursor)
		if a.qdata.Len() > 0 && realIdx < a.qdata.Len() {
			t := a.qdata.Tracks[realIdx]
			seed = &t
		}
	case panelHistory:
		seed = a.history.currentTrack()
	case panelPlaylist:
		if a.playlist.level == levelDetail {
			tracks := a.playlist.visibleTracks()
			if len(tracks) > 0 && a.playlist.detailCur < len(tracks) {
				t := tracks[a.playlist.detailCur]
				seed = &t
			}
		}
	case panelArtists:
		if a.artistsLevel == 2 {
			tracks := a.artistsFilteredTracks()
			if len(tracks) > 0 && a.artistsPanelCur < len(tracks) {
				t := tracks[a.artistsPanelCur]
				seed = &t
			}
		}
	}

	if seed == nil {
		return a, nil
	}

	if a.radioLoading {
		return a, nil // already loading
	}

	// Launch async yt-dlp search for radio mix
	a.radioLoading = true
	seedCopy := *seed
	cmd := a.setStatus(fmt.Sprintf("Loading radio: %s - %s...", seed.Artist, seed.Title))
	radioCmd := func() tea.Msg {
		tracks, err := youtube.RadioMix(seedCopy)
		return radioResultMsg{seed: seedCopy, tracks: tracks, err: err}
	}
	return a, tea.Batch(cmd, radioCmd)
}

func (a App) handleToggleFavorite() (tea.Model, tea.Cmd) {
	fav := a.playlist.store.Favorites()
	if fav == nil {
		// Auto-create the Favorites playlist if it was deleted
		var err error
		fav, err = a.playlist.store.Create("Favorites")
		if err != nil {
			cmd := a.setStatus("Failed to create Favorites playlist")
			return a, cmd
		}
	}

	// Gather tracks: visual selection or single track under cursor
	var tracks []model.Track
	switch a.focusedPanel {
	case panelSearch:
		if a.search.visual {
			visible := a.search.visibleTracks()
			lo, hi := a.search.anchor, a.search.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi && i < len(visible); i++ {
				tracks = append(tracks, visible[i])
			}
			a.search.visual = false
		} else if t := a.search.currentTrack(); t != nil {
			tracks = append(tracks, *t)
		}
	case panelQueue:
		if a.queue.visual {
			visible := a.queue.visibleTracks(a.qdata)
			lo, hi := a.queue.anchor, a.queue.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi && i < len(visible); i++ {
				tracks = append(tracks, visible[i])
			}
			a.queue.visual = false
		} else {
			realIdx := a.queue.realIndex(a.queue.cursor)
			if a.qdata.Len() > 0 && realIdx < a.qdata.Len() {
				tracks = append(tracks, a.qdata.Tracks[realIdx])
			}
		}
	case panelHistory:
		if a.history.visual {
			allTracks := a.history.tracks()
			lo, hi := a.history.anchor, a.history.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi && i < len(allTracks); i++ {
				tracks = append(tracks, allTracks[i])
			}
			a.history.visual = false
		} else if t := a.history.currentTrack(); t != nil {
			tracks = append(tracks, *t)
		}
	case panelPlaylist:
		if a.playlist.visual && a.playlist.level == levelDetail {
			visible := a.playlist.visibleTracks()
			lo, hi := a.playlist.anchor, a.playlist.detailCur
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi && i < len(visible); i++ {
				tracks = append(tracks, visible[i])
			}
			a.playlist.visual = false
		} else {
			visible := a.playlist.visibleTracks()
			if a.playlist.level == levelDetail && a.playlist.detailCur < len(visible) {
				tracks = append(tracks, visible[a.playlist.detailCur])
			}
		}
	}

	if len(tracks) == 0 {
		return a, nil
	}

	// Snapshot favorites for undo
	favIdx := -1
	for i, p := range a.playlist.store.Playlists {
		if p.Name == "Favorites" {
			favIdx = i
			break
		}
	}
	if favIdx >= 0 {
		snap := make([]model.Track, len(fav.Tracks))
		copy(snap, fav.Tracks)
		a.undoStack = append(a.undoStack, undoEntry{
			kind:        undoFavorite,
			playlistIdx: favIdx,
			tracks:      snap,
		})
		a.redoStack = nil
	}

	// Toggle each track individually: fav→unfav, unfav→fav
	var added, removed int
	for _, t := range tracks {
		if fav.ContainsTrack(t.ID) {
			fav.RemoveTrackByID(t.ID)
			removed++
		} else {
			_ = fav.AddTracks(t)
			added++
		}
	}
	var statusMsg string
	if len(tracks) == 1 {
		if removed > 0 {
			statusMsg = fmt.Sprintf("Removed from favorites: %s", tracks[0].Title)
		} else {
			statusMsg = fmt.Sprintf("Added to favorites: %s", tracks[0].Title)
		}
	} else {
		var parts []string
		if added > 0 {
			parts = append(parts, fmt.Sprintf("%d added", added))
		}
		if removed > 0 {
			parts = append(parts, fmt.Sprintf("%d removed", removed))
		}
		statusMsg = fmt.Sprintf("Favorites: %s", strings.Join(parts, ", "))
	}
	cmd := a.setStatus(statusMsg)
	return a, cmd
}

func (a App) handleAddToPlaylist() (tea.Model, tea.Cmd) {
	var tracks []model.Track

	switch a.focusedPanel {
	case panelSearch:
		if a.search.visual {
			visible := a.search.visibleTracks()
			lo, hi := a.search.anchor, a.search.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi && i < len(visible); i++ {
				tracks = append(tracks, visible[i])
			}
			a.search.visual = false
		} else if t := a.search.currentTrack(); t != nil {
			tracks = append(tracks, *t)
		}

	case panelQueue:
		visible := a.queue.visibleTracks(a.qdata)
		if a.queue.visual {
			lo, hi := a.queue.anchor, a.queue.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi && i < len(visible); i++ {
				tracks = append(tracks, visible[i])
			}
			a.queue.visual = false
		} else if a.queue.cursor < len(visible) {
			tracks = append(tracks, visible[a.queue.cursor])
		}

	case panelHistory:
		visible := a.history.tracks()
		if a.history.visual {
			lo, hi := a.history.anchor, a.history.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi && i < len(visible); i++ {
				tracks = append(tracks, visible[i])
			}
			a.history.visual = false
		} else if t := a.history.currentTrack(); t != nil {
			tracks = append(tracks, *t)
		}

	case panelArtists:
		if a.artistsLevel == 0 {
			return a.handleFollowArtist()
		}
		if a.artistsLevel == 1 {
			// Album list: add all tracks from album to queue
			if a.artistsPanelLoad || a.artistsPanelCur >= len(a.artistsPanelAlbs) {
				return a, nil
			}
			album := a.artistsPanelAlbs[a.artistsPanelCur]
			artistIdx := a.artistStoreIdxByName(a.artistsPanelName)
			if artistIdx >= 0 {
				for _, sa := range a.artistStore.Artists[artistIdx].Albums {
					if sa.ID == album.ID && len(sa.Tracks) > 0 {
						cached := savedAlbumTracksToModel(sa.Tracks)
						a.saveQueueUndo()
						a.qdata.Add(cached...)
						a.queue.cursor = a.qdata.Len() - 1
						cmd := a.setStatus(fmt.Sprintf("Added %d tracks from \"%s\" to queue", len(cached), album.Title))
						return a, cmd
					}
				}
			}
			a.artistsPanelLoad = true
			a.artistsPanelAlbN = album.Title
			cmd := a.setStatus(fmt.Sprintf("Fetching \"%s\"...", album.Title))
			fetchCmd := func() tea.Msg {
				trks, err := youtube.FetchAlbumTracks(album)
				return artistAddAlbumMsg{album: album, tracks: trks, err: err}
			}
			return a, tea.Batch(cmd, fetchCmd)
		}
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
			if len(tracks) > 0 {
				a.saveQueueUndo()
				a.qdata.Add(tracks...)
				a.queue.cursor = a.qdata.Len() - 1
				cmd := a.setStatus(fmt.Sprintf("Added %d tracks to queue", len(tracks)))
				return a, cmd
			}
			return a, nil
		}

	case panelPlaylist:
		if a.playlist.level == levelList {
			// List level: add all tracks from selected playlist(s) directly to queue
			pls := a.playlist.visiblePlaylists()
			if a.playlist.listVisual {
				lo, hi := a.playlist.listAnchor, a.playlist.listCur
				if lo > hi {
					lo, hi = hi, lo
				}
				for i := lo; i <= hi && i < len(pls); i++ {
					tracks = append(tracks, pls[i].Tracks...)
				}
				a.playlist.listVisual = false
			} else if a.playlist.listCur < len(pls) {
				tracks = append(tracks, pls[a.playlist.listCur].Tracks...)
			}
			if len(tracks) == 0 {
				return a, nil
			}
			a.saveQueueUndo()
			a.qdata.Add(tracks...)
			a.queue.cursor = a.qdata.Len() - 1
			var name string
			if a.playlist.listCur < len(pls) {
				name = pls[a.playlist.listCur].Name
			}
			cmd := a.setStatus(fmt.Sprintf("Added %d tracks from %s to queue", len(tracks), name))
			return a, cmd
		}
		if a.playlist.level == levelDetail {
			visible := a.playlist.visibleTracks()
			if a.playlist.visual {
				lo, hi := a.playlist.anchor, a.playlist.detailCur
				if lo > hi {
					lo, hi = hi, lo
				}
				for i := lo; i <= hi && i < len(visible); i++ {
					tracks = append(tracks, visible[i])
				}
				a.playlist.visual = false
			} else if a.playlist.detailCur < len(visible) {
				tracks = append(tracks, visible[a.playlist.detailCur])
			}
		}
	}

	if len(tracks) == 0 {
		return a, nil
	}
	a.overlay.open(a.playlist.store, tracks)
	return a, nil
}

func (a App) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If in create mode, handle text input
	if a.overlay.creating {
		switch {
		case key.Matches(msg, keys.Escape):
			a.overlay.creating = false
			a.overlay.input.Blur()
			return a, nil
		case key.Matches(msg, keys.Enter):
			if plName, ok := a.overlay.confirmCreate(); ok {
				trackCount := len(a.overlay.tracks)
				statusMsg := fmt.Sprintf("%d song", trackCount)
				if trackCount != 1 {
					statusMsg += "s"
				}
				statusMsg += " added to " + plName
				a.overlay.close()
				cmd := a.setStatus(statusMsg)
				return a, cmd
			}
			return a, nil
		default:
			var cmd tea.Cmd
			a.overlay.input, cmd = a.overlay.input.Update(msg)
			return a, cmd
		}
	}

	switch {
	case key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c":
		a.quit()
		return a, tea.Quit
	case key.Matches(msg, keys.Up):
		a.overlay.moveUp()
	case key.Matches(msg, keys.Down):
		a.overlay.moveDown()
	case key.Matches(msg, keys.HalfDown):
		a.overlay.halfDown()
		return a, nil
	case key.Matches(msg, keys.HalfUp):
		a.overlay.halfUp()
		return a, nil
	case msg.String() == "g":
		a.overlay.goTop()
		return a, nil
	case msg.String() == "G":
		a.overlay.goBottom()
		return a, nil
	case key.Matches(msg, keys.Visual):
		a.overlay.visual = !a.overlay.visual
		if a.overlay.visual {
			a.overlay.anchor = a.overlay.cursor
		}
		return a, nil
	case msg.String() == " ":
		createIdx := 1 + len(a.overlay.playlists)
		if a.overlay.visual {
			// Select all in visual range
			lo, hi := a.overlay.anchor, a.overlay.cursor
			if lo > hi {
				lo, hi = hi, lo
			}
			for i := lo; i <= hi; i++ {
				if i < createIdx { // don't select "Create new"
					a.overlay.selected[i] = true
				}
			}
			a.overlay.visual = false
		} else if a.overlay.cursor < createIdx {
			// Toggle single item
			if a.overlay.selected[a.overlay.cursor] {
				delete(a.overlay.selected, a.overlay.cursor)
			} else {
				a.overlay.selected[a.overlay.cursor] = true
			}
		}
		return a, nil
	case key.Matches(msg, keys.Enter):
		trackCount := len(a.overlay.tracks)
		// If multi-select is active, add to all selected playlists
		if len(a.overlay.selected) > 0 {
			var names []string
			for idx := range a.overlay.selected {
				if idx == 0 {
					// Queue
					a.saveQueueUndo()
					a.qdata.Add(a.overlay.tracks...)
					a.queue.cursor = a.qdata.Len() - 1
					names = append(names, "Queue")
				} else {
					plIdx := idx - 1
					if plIdx >= 0 && plIdx < len(a.overlay.playlists) {
						p := a.overlay.playlists[plIdx]
						// Find the real store index for undo
						for si, sp := range a.playlist.store.Playlists {
							if sp == p {
								snap := make([]model.Track, len(p.Tracks))
								copy(snap, p.Tracks)
								a.undoStack = append(a.undoStack, undoEntry{
									kind:        undoTracks,
									playlistIdx: si,
									tracks:      snap,
									cursor:      a.playlist.detailCur,
								})
								break
							}
						}
						_ = p.AddTracks(a.overlay.tracks...)
						names = append(names, p.Name)
					}
				}
			}
			a.redoStack = nil
			a.overlay.close()
			statusMsg := fmt.Sprintf("%d song", trackCount)
			if trackCount != 1 {
				statusMsg += "s"
			}
			statusMsg += " added to " + strings.Join(names, ", ")
			cmd := a.setStatus(statusMsg)
			return a, cmd
		}
		if a.overlay.isQueueSelected() {
			// Add to queue
			a.saveQueueUndo()
			a.qdata.Add(a.overlay.tracks...)
			a.queue.cursor = a.qdata.Len() - 1
			a.overlay.close()
			statusMsg := fmt.Sprintf("%d song", trackCount)
			if trackCount != 1 {
				statusMsg += "s"
			}
			statusMsg += " added to Queue"
			cmd := a.setStatus(statusMsg)
			return a, cmd
		}
		if a.overlay.isCreateSelected() {
			cmd := a.overlay.startCreate()
			return a, cmd
		}
		if plIdx := a.overlay.cursor - 1; plIdx >= 0 && plIdx < len(a.overlay.playlists) {
			// Save undo before adding tracks
			p := a.overlay.playlists[plIdx]
			for si, sp := range a.playlist.store.Playlists {
				if sp == p {
					snap := make([]model.Track, len(p.Tracks))
					copy(snap, p.Tracks)
					a.undoStack = append(a.undoStack, undoEntry{
						kind:        undoTracks,
						playlistIdx: si,
						tracks:      snap,
						cursor:      a.playlist.detailCur,
					})
					a.redoStack = nil
					break
				}
			}
		}
		if plName, ok := a.overlay.confirmPlaylist(); ok {
			statusMsg := fmt.Sprintf("%d song", trackCount)
			if trackCount != 1 {
				statusMsg += "s"
			}
			statusMsg += " added to " + plName
			a.overlay.close()
			cmd := a.setStatus(statusMsg)
			return a, cmd
		}
	case key.Matches(msg, keys.Escape):
		a.overlay.close()
	}
	return a, nil
}
