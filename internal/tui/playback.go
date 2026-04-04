package tui

import (
	"math/rand"

	"github.com/Sadoaz/vimyt/internal/model"
)

// syncRadioToQueue replaces the queue with current radio tracks.
func (a *App) syncRadioToQueue() {
	cur := a.qdata.Current
	a.qdata.Clear()
	a.qdata.Add(a.playlist.radioTracks...)
	// Try to preserve current playing position
	if cur >= 0 && cur < a.qdata.Len() {
		a.qdata.Current = cur
	} else if a.qdata.Len() > 0 {
		a.qdata.Current = 0
	}
}

// pickNextTrack determines the index of the next track to play and updates shuffle state.
// Uses shuffle-without-repeats when shuffle is on, sequential wrap otherwise.
func (a *App) pickNextTrack() int {
	if a.qdata.Len() == 0 {
		return 0
	}
	if a.shuffle {
		return a.pickShuffleNext()
	}
	// Sequential: wrap around
	next := a.qdata.Current + 1
	if next >= a.qdata.Len() {
		next = 0
	}
	return next
}

// peekNextTrack returns the index of the next track WITHOUT modifying shuffle state.
// Used for prefetch — we want to know which track is next but not commit to it.
func (a *App) peekNextTrack() int {
	if a.qdata.Len() == 0 {
		return 0
	}
	if a.shuffle {
		return a.peekShuffleNext()
	}
	next := a.qdata.Current + 1
	if next >= a.qdata.Len() {
		next = 0
	}
	return next
}

// peekShuffleNext returns a random unplayed track index WITHOUT modifying shufflePlayed.
func (a *App) peekShuffleNext() int {
	n := a.qdata.Len()
	if n == 0 {
		return 0
	}

	played := a.shufflePlayed
	if played == nil {
		played = make(map[string]bool)
	}

	// Include current track as "played" for candidate filtering
	currentID := ""
	if a.qdata.Current >= 0 && a.qdata.Current < n {
		currentID = a.qdata.Tracks[a.qdata.Current].ID
	}

	var candidates []int
	for i := range n {
		id := a.qdata.Tracks[i].ID
		if !played[id] && id != currentID {
			candidates = append(candidates, i)
		}
	}

	if len(candidates) == 0 {
		// All played — pick from all except current
		for i := range n {
			if a.qdata.Tracks[i].ID != currentID || n == 1 {
				candidates = append(candidates, i)
			}
		}
	}

	if len(candidates) == 0 {
		return 0
	}
	return candidates[rand.Intn(len(candidates))]
}

// pickShuffleNext picks a random track that hasn't been played yet.
// Resets the played set when all tracks have been played.
func (a *App) pickShuffleNext() int {
	n := a.qdata.Len()
	if n == 0 {
		return 0
	}
	if a.shufflePlayed == nil {
		a.shufflePlayed = make(map[string]bool)
	}

	// Mark current track as played
	if a.qdata.Current >= 0 && a.qdata.Current < n {
		a.shufflePlayed[a.qdata.Tracks[a.qdata.Current].ID] = true
	}

	// Build list of unplayed indices
	var candidates []int
	for i := range n {
		if !a.shufflePlayed[a.qdata.Tracks[i].ID] {
			candidates = append(candidates, i)
		}
	}

	// All played — reset and pick from all (except current if possible)
	if len(candidates) == 0 {
		a.shufflePlayed = make(map[string]bool)
		for i := range n {
			candidates = append(candidates, i)
		}
	}

	// Pick random from candidates
	idx := candidates[rand.Intn(len(candidates))]
	// Mark as played
	a.shufflePlayed[a.qdata.Tracks[idx].ID] = true
	return idx
}

// pushPrev saves the current playing index onto the prev stack.
// Must be called BEFORE qdata.Current is changed.
func (a *App) pushPrev() {
	if a.qdata.Current >= 0 && a.qdata.Current < a.qdata.Len() {
		a.prevStack = append(a.prevStack, a.qdata.Current)
		if len(a.prevStack) > 200 {
			a.prevStack = a.prevStack[len(a.prevStack)-200:]
		}
	}
}

// pushJump records the current panel onto the jump back stack.
// Should be called BEFORE changing focusedPanel.
func (a *App) pushJump(from panel) {
	// Don't push consecutive duplicates
	if len(a.jumpBack) > 0 && a.jumpBack[len(a.jumpBack)-1] == from {
		return
	}
	a.jumpBack = append(a.jumpBack, from)
	if len(a.jumpBack) > 50 {
		a.jumpBack = a.jumpBack[len(a.jumpBack)-50:]
	}
	// Clear forward stack on new navigation
	a.jumpFwd = nil
}

// playTrack starts playing a track and records it in play history.
func (a *App) playTrack(t *model.Track, source string) {
	a.player.Play(t)
	if a.playHistory != nil {
		a.playHistory.Add(*t, source)
	}
}

// cancelPrefetch cancels any in-flight prefetch and resets the prefetch state.
func (a *App) cancelPrefetch() {
	if a.prefetchCancel != nil {
		a.prefetchCancel()
		a.prefetchCancel = nil
	}
	a.prefetchedID = ""
	a.prefetchNextIdx = -1
}

// quit saves session and cleans up before exiting.
func (a *App) quit() {
	a.cancelPrefetch()
	a.saveSession()
	_ = model.SaveQueue(a.qdata)
	a.player.Close()
}

// saveSession persists current UI state to disk.
func (a *App) saveSession() {
	// Get current playback position
	status := a.player.Status()
	var playPos float64
	if status.State == model.Playing || status.State == model.Paused {
		playPos = status.Position.Seconds()
	}

	s := model.Session{
		View:           int(a.focusedPanel),
		SearchQuery:    a.search.input.Value(),
		SearchCur:      a.search.cursor,
		QueueCur:       a.queue.cursor,
		HistoryCur:     a.history.cursor,
		RadioHistCur:   a.radioHistCur,
		PLListCur:      a.playlist.listCur,
		PLDetailCur:    a.playlist.detailCur,
		Zoomed:         a.zoomed,
		PlaybackPos:    playPos,
		RadioActive:    a.radioActive,
		RadioSeed:      a.radioSeedTitle,
		Volume:         status.Volume,
		Autoplay:       a.autoplay,
		Shuffle:        a.shuffle,
		PinSearch:      a.pinSearch,
		PinPlaylist:    a.pinPlaylist,
		ShowHistory:    a.showHistory,
		ShowRadio:      a.showRadio,
		PinRadio:       a.pinRadio,
		RelNumbers:     a.relNumbers,
		AutoFocusQueue: a.autoFocusQueue,
		CookieBrowser:  a.cookieBrowser,
		ShowArtists:    a.showArtistsPanel,
		PinArtists:     a.pinArtists,
		ArtistsCur:     a.artistsPanelCur,
		LoopTrack:      a.loopTrack,
		LoopCount:      a.loopCount,
		LoopTotal:      a.loopTotal,
		Theme:          a.theme.ToMap(),
	}
	if a.playlist.radioActive {
		// Don't save radio as detail level — go back to list
		s.PLLevel = 0
	} else {
		s.PLLevel = int(a.playlist.level)
	}
	_ = model.SaveSession(s)
}
