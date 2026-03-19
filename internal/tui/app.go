// Package tui implements the terminal user interface for vimyt.
package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sadoaz/vimyt/internal/model"
	"github.com/Sadoaz/vimyt/internal/player"
	"github.com/Sadoaz/vimyt/internal/youtube"
)

type panel int

const (
	panelSearch panel = iota
	panelQueue
	panelPlaylist
	panelHistory
	panelRadioHist
)

// App is the root Bubble Tea model.
type App struct {
	search          searchModel
	queue           queueModel
	playlist        playlistModel
	history         historyModel
	overlay         overlayModel
	qdata           *model.Queue
	player          *player.Player
	focusedPanel    panel
	prevPanel       panel
	width           int
	height          int
	showHelp        bool
	helpScroll      int
	helpFilter      string
	helpFilterInput textinput.Model
	helpFiltering   bool
	// GOTO mode: press g to open GOTO input, type time to seek or g to go to top
	gotoInput  textinput.Model
	gotoActive bool
	// yy tracking: waiting for second 'y'
	waitingY bool
	// :number jump command
	colonInput  textinput.Model
	colonActive bool
	// dd tracking: waiting for second 'd'
	waitingD bool
	// Track clipboard for x (cut) and p (paste)
	clipboard           []model.Track
	clipboardName       string // name hint for pasting (e.g. source playlist name)
	clipboardHadCurrent bool   // true if cut removed the currently-playing track from queue
	clipboardCurrentOff int    // offset of the playing track within clipboard
	// Playlist clipboard for cut/paste of entire playlists
	plClipboard []struct {
		name   string
		tracks []model.Track
	}
	// Undo/redo stacks
	undoStack []undoEntry
	redoStack []undoEntry

	// Radio mode
	radioActive          bool   // true when queue contains a radio mix
	radioSeedTitle       string // title of the seed track (shown in [radio] tag)
	radioLoading         bool   // true while waiting for yt-dlp radio results
	radioHistory         *model.RadioHistory
	radioHistCur         int // cursor in radio history panel
	radioHistScroll      int // scroll offset for radio history viewport
	radioHistFilter      string
	radioHistFilterInput textinput.Model
	radioHistFiltering   bool
	radioHistWaitD       bool                        // waiting for second 'd' in dd
	radioHistClipboard   []model.RadioHistoryEntry   // clipboard for radio history entries
	radioHistVisual      bool                        // visual select in radio history
	radioHistAnchor      int                         // anchor for visual select
	radioHistUndo        [][]model.RadioHistoryEntry // undo stack for radio history
	radioHistRedo        [][]model.RadioHistoryEntry // redo stack for radio history

	// Transient status message shown in the bottom bar
	statusMsg    string
	statusExpiry time.Time
	// Dependency check error (shown once on first render)
	depErr string
	// Play history — all tracks played
	playHistory *model.PlayHistory
	// Zoom: when true the focused panel takes full content area
	zoomed bool
	// Resume playback: seconds to seek to on startup (0 = no resume)
	resumePos float64
	// Tick counter for marquee animation (incremented every playerTick = 500ms)
	tickCount int
	// Settings
	autoplay            bool            // auto-advance to next track on EOF
	shuffle             bool            // randomize next track selection
	pinSearch           bool            // keep search panel expanded when unfocused
	pinPlaylist         bool            // keep playlist detail expanded when unfocused
	showHistory         bool            // show history panel below playlists
	showRadio           bool            // show radio history panel
	pinRadio            bool            // keep radio history expanded when unfocused
	relNumbers          bool            // show relative line numbers (vim-style)
	autoFocusQueue      bool            // focus queue panel when playing a track
	cookieBrowser       string          // browser for yt-dlp cookie auth ("" = off)
	showSettings        bool            // settings overlay visible
	settingsCur         int             // cursor in settings list
	settingsImporting   bool            // true when URL input is active in settings
	settingsImportInput textinput.Model // text input for playlist URL
	importingPlaylist   bool            // true while async import is running
	// Prefetch: ID of the track whose URL is already being resolved ahead of time
	prefetchedID string
	// Shuffle tracking: set of played track IDs to avoid repeats
	shufflePlayed map[string]bool
	// Play-back stack: queue indices of previously played tracks (for Shift+P)
	prevStack []int
	// Vim-style jumplist for panel focus changes
	jumpBack []panel // back stack
	jumpFwd  []panel // forward stack
}

// clearStatusMsg is sent after the status message timeout expires.
type clearStatusMsg struct{}

// playerTickMsg is sent periodically to poll mpv status and update the now-playing bar.
type playerTickMsg time.Time

// radioResultMsg carries results back from async radio mix generation.
type radioResultMsg struct {
	seed   model.Track
	tracks []model.Track
	err    error
}

// importPlaylistMsg carries results back from async playlist import.
type importPlaylistMsg struct {
	name   string
	tracks []model.Track
	err    error
}

// setStatus sets a transient status message that auto-clears after a few seconds.
func (a *App) setStatus(msg string) tea.Cmd {
	a.statusMsg = msg
	a.statusExpiry = time.Now().Add(3 * time.Second)
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// playerTick returns a command that schedules the next player status poll.
func playerTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return playerTickMsg(t)
	})
}

// checkDeps verifies yt-dlp and mpv are available on PATH.
func checkDeps() string {
	var missing []string
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		missing = append(missing, "yt-dlp")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		missing = append(missing, "mpv")
	}
	if len(missing) > 0 {
		return fmt.Sprintf("Missing required programs: %s", strings.Join(missing, ", "))
	}
	return ""
}

// New creates a new App model, restoring previous session state.
func New(plStore *model.PlaylistStore) App {
	ci := textinput.New()
	ci.Prompt = ":"
	ci.CharLimit = 10
	ci.Cursor.SetMode(cursor.CursorStatic)

	gi := textinput.New()
	gi.Prompt = "GOTO: "
	gi.Placeholder = "e.g. 30, 1:23, 1:23:45"
	gi.CharLimit = 12
	gi.Cursor.SetMode(cursor.CursorStatic)

	hi := textinput.New()
	hi.Prompt = "/"
	hi.CharLimit = 40

	ri := textinput.New()
	ri.CharLimit = 80
	ri.Cursor.SetMode(cursor.CursorStatic)

	ii := textinput.New()
	ii.Prompt = "URL: "
	ii.Placeholder = "https://youtube.com/playlist?list=..."
	ii.CharLimit = 256
	ii.Cursor.SetMode(cursor.CursorStatic)
	hi.Cursor.SetMode(cursor.CursorStatic)

	sessionExists := model.SessionExists()
	sess := model.LoadSession()
	// Default settings to true for new sessions (Go zero value is false).
	if !sessionExists {
		sess.Autoplay = true
		sess.Shuffle = true
		sess.ShowHistory = true
		sess.ShowRadio = true
		sess.AutoFocusQueue = true
		sess.RelNumbers = true
	}

	sm := newSearchModel()
	qm := newQueueModel()
	pm := newPlaylistModel(plStore)

	v := panel(sess.View)
	if v < panelSearch || v > panelRadioHist {
		v = panelSearch
	}
	// If history panel was focused but is now hidden, redirect to playlists
	if v == panelHistory && !sess.ShowHistory {
		v = panelPlaylist
	}
	// If radio history panel was focused but is now hidden, redirect to playlists
	if v == panelRadioHist && !sess.ShowRadio {
		v = panelPlaylist
	}

	// Restore search state
	if sess.SearchQuery != "" {
		sm.input.SetValue(sess.SearchQuery)
		// Re-run the search to populate results
		tracks, _ := youtube.Search(sess.SearchQuery)
		sm.results = tracks
		sm.cursor = sess.SearchCur
		sm.cursor = min(sm.cursor, len(sm.results)-1)
		sm.cursor = max(sm.cursor, 0)
	}

	ph := model.LoadPlayHistory()
	hm := newHistoryModel(ph)
	// Restore history cursor
	hm.cursor = sess.HistoryCur
	hm.cursor = min(hm.cursor, ph.Len()-1)
	hm.cursor = max(hm.cursor, 0)

	// Restore playlist state
	pm.listCur = sess.PLListCur
	total := pm.totalListLen()
	pm.listCur = min(pm.listCur, total-1)
	pm.listCur = max(pm.listCur, 0)
	if sess.PLLevel == 1 && pm.currentPlaylist() != nil {
		pm.level = levelDetail
		pm.detailCur = sess.PLDetailCur
		pm.detailCur = min(pm.detailCur, len(pm.currentPlaylist().Tracks)-1)
		pm.detailCur = max(pm.detailCur, 0)
	}

	qdata := model.LoadQueue()

	// Restore queue cursor, clamping to loaded queue size
	qm.cursor = sess.QueueCur
	qm.cursor = min(qm.cursor, qdata.Len()-1)
	qm.cursor = max(qm.cursor, 0)

	app := App{
		search:               sm,
		queue:                qm,
		playlist:             pm,
		history:              hm,
		overlay:              newOverlayModel(),
		qdata:                qdata,
		player:               player.New(),
		focusedPanel:         v,
		colonInput:           ci,
		gotoInput:            gi,
		helpFilterInput:      hi,
		radioHistFilterInput: ri,
		settingsImportInput:  ii,
		radioHistory:         model.LoadRadioHistory(),
		playHistory:          ph,
		depErr:               checkDeps(),
		zoomed:               sess.Zoomed,
		radioActive:          sess.RadioActive,
		radioSeedTitle:       sess.RadioSeed,
		resumePos:            sess.PlaybackPos,
		autoplay:             sess.Autoplay,
		shuffle:              sess.Shuffle,
		pinSearch:            sess.PinSearch,
		pinPlaylist:          sess.PinPlaylist,
		showHistory:          sess.ShowHistory,
		showRadio:            sess.ShowRadio,
		pinRadio:             sess.PinRadio,
		relNumbers:           sess.RelNumbers,
		autoFocusQueue:       sess.AutoFocusQueue,
		cookieBrowser:        sess.CookieBrowser,
	}
	// Apply cookie browser setting to youtube package
	youtube.SetCookieBrowser(app.cookieBrowser)
	// Restore volume from session (only if > 0 to avoid overriding default)
	if sess.Volume > 0 {
		app.player.SetVolume(sess.Volume)
	}
	// Restore radio history cursor
	app.radioHistCur = sess.RadioHistCur
	rhVisible, _ := app.radioHistVisible()
	app.radioHistCur = min(app.radioHistCur, len(rhVisible)-1)
	app.radioHistCur = max(app.radioHistCur, 0)
	return app
}

func (a App) Init() tea.Cmd {
	cmds := []tea.Cmd{playerTick()}
	// Resume playback from last session — load paused and seek (no audio heard)
	if a.qdata.Current >= 0 && a.qdata.Current < a.qdata.Len() && a.resumePos > 0 {
		t := &a.qdata.Tracks[a.qdata.Current]
		a.player.PlayPaused(t, a.resumePos)
	}
	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearStatusMsg:
		// Only clear if the message has actually expired
		if time.Now().After(a.statusExpiry) {
			a.statusMsg = ""
		}
		return a, nil

	case playerTickMsg:
		a.tickCount++
		// Surface any player errors (e.g. URL resolve failures) as status messages
		if errMsg := a.player.PopErr(); errMsg != "" {
			cmd := a.setStatus(errMsg)
			return a, tea.Batch(cmd, playerTick())
		}

		// Poll mpv status — Status() queries mpv IPC for real position/state.
		// The player detects EOF internally and transitions to Stopped.
		status := a.player.Status()

		// Auto-advance: if player stopped (track ended) and autoplay is enabled
		if status.State == model.Stopped && status.Track != nil && a.autoplay {
			if a.qdata.Len() > 0 {
				a.pushPrev()
				idx := a.pickNextTrack()
				a.qdata.Current = idx
				a.playTrack(&a.qdata.Tracks[idx], "queue")
				a.prefetchedID = "" // reset prefetch after advancing
			}
		}

		// Prefetch: resolve URL for next track ~15s before current ends
		if status.State == model.Playing && status.Track != nil && a.autoplay && a.qdata.Len() > 0 {
			dur := status.Track.Duration
			pos := status.Position
			if dur > 0 && pos > 0 && dur-pos <= 15*time.Second {
				nextIdx := a.peekNextTrack()
				nextTrack := a.qdata.Tracks[nextIdx]
				if nextTrack.ID != a.prefetchedID {
					a.prefetchedID = nextTrack.ID
					go youtube.ResolveURL(nextTrack.ID)
				}
			}
		}
		return a, playerTick()
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Panel layout dimensions for sub-models — approximate for scroll calc.
		// Actual rendering uses dynamic search height in View().
		contentHeight := msg.Height - 2 // status bar (1) + now-playing bar (1)
		searchH := contentHeight / 2
		bottomH := contentHeight - searchH
		leftW := msg.Width * 40 / 100
		rightW := msg.Width - leftW
		plH := bottomH / 3
		histH := bottomH / 3
		// radioHistH would be bottomH - plH - histH but not stored as sub-model
		a.search.height = searchH - 2
		a.search.width = msg.Width - 2
		a.queue.height = bottomH - 2
		a.queue.width = rightW - 2
		a.playlist.height = plH - 2
		a.playlist.width = leftW - 2
		a.history.height = histH - 2
		a.history.width = leftW - 2
		return a, nil

	case searchResultMsg:
		var cmd tea.Cmd
		a.search, cmd = a.search.Update(msg)
		return a, cmd

	case radioResultMsg:
		a.radioLoading = false
		if msg.err != nil || len(msg.tracks) == 0 {
			cmd := a.setStatus("Radio: no results found")
			return a, cmd
		}

		// Check if the seed track is already playing — if so, don't restart it
		currentlyPlaying := a.player.Status()
		seedPlaying := currentlyPlaying.Track != nil && currentlyPlaying.Track.ID == msg.seed.ID

		// Save current queue state to jumplist before replacing

		a.saveQueueUndo()
		a.qdata.Clear()
		a.qdata.Add(msg.tracks...)
		a.qdata.Current = 0
		a.shufflePlayed = nil
		a.prefetchedID = ""
		if !seedPlaying {
			a.playTrack(&a.qdata.Tracks[0], "radio")
		}
		a.queue.cursor = 0

		// Mark radio mode on the app
		a.radioActive = true
		a.radioSeedTitle = msg.seed.Title

		// Clear any previous playlist radio state (radio lives in queue only)
		a.playlist.dismissRadio()

		// Switch focus to Queue to show the radio mix
		a.pushJump(a.focusedPanel)
		a.prevPanel = a.focusedPanel
		a.focusedPanel = panelQueue

		// Record in radio history (with full track list for recovery)
		a.radioHistory.Add(msg.seed.Title, msg.seed.Artist, len(msg.tracks), msg.tracks)

		cmd := a.setStatus(fmt.Sprintf("%d songs added to radio", len(msg.tracks)))
		return a, cmd

	case importPlaylistMsg:
		a.importingPlaylist = false
		if msg.err != nil {
			cmd := a.setStatus(fmt.Sprintf("Import failed: %v", msg.err))
			return a, cmd
		}
		name := msg.name
		if name == "" {
			name = "Imported Playlist"
		}
		pl, err := a.playlist.store.Create(name)
		if err != nil {
			cmd := a.setStatus(fmt.Sprintf("Import failed: %v", err))
			return a, cmd
		}
		_ = pl.AddTracks(msg.tracks...)
		a.playlist.listCur = len(a.playlist.store.Playlists) - 1
		cmd := a.setStatus(fmt.Sprintf("Imported \"%s\" with %d tracks", name, len(msg.tracks)))
		return a, cmd
	}

	// If help overlay is shown, handle navigation/search
	if a.showHelp {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateHelp(msg)
		}
	}

	// If settings overlay is shown, handle navigation and toggles
	if a.showSettings {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateSettings(msg)
		}
		// Forward non-key messages to import input for cursor blink
		if a.settingsImporting {
			var cmd tea.Cmd
			a.settingsImportInput, cmd = a.settingsImportInput.Update(msg)
			return a, cmd
		}
		return a, nil
	}

	// If radio history filter input is active, handle it
	if a.radioHistFiltering {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateRadioHistFilter(msg)
		}
		var cmd tea.Cmd
		a.radioHistFilterInput, cmd = a.radioHistFilterInput.Update(msg)
		return a, cmd
	}

	// If add-to-playlist overlay is active, handle it
	if a.overlay.active {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateOverlay(msg)
		}
		return a, nil
	}

	// If playlist inline input is active (create/rename/filter), handle it
	if a.playlist.inputMode != plInputNone {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updatePlaylistInput(msg)
		}
		// Pass non-key messages to text input
		var cmd tea.Cmd
		a.playlist.input, cmd = a.playlist.input.Update(msg)
		return a, cmd
	}

	// If queue filter input is active, handle it
	if a.queue.isFilterActive() {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateQueueFilter(msg)
		}
		// Pass non-key messages to text input
		var cmd tea.Cmd
		a.queue.filterInput, cmd = a.queue.filterInput.Update(msg)
		return a, cmd
	}

	// If search filter input is active, handle it
	if a.search.isFilterActive() {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateSearchFilter(msg)
		}
		// Pass non-key messages to text input
		var cmd tea.Cmd
		a.search.filterInput, cmd = a.search.filterInput.Update(msg)
		return a, cmd
	}

	// If history filter input is active, handle it
	if a.history.isFilterActive() {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateHistoryFilter(msg)
		}
		var cmd tea.Cmd
		a.history.filterInput, cmd = a.history.filterInput.Update(msg)
		return a, cmd
	}

	// If GOTO input is active, handle it
	if a.gotoActive {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateGotoInput(msg)
		}
		var cmd tea.Cmd
		a.gotoInput, cmd = a.gotoInput.Update(msg)
		return a, cmd
	}

	// If colon input is active, handle it
	if a.colonActive {
		if msg, ok := msg.(tea.KeyMsg); ok {
			return a.updateColonInput(msg)
		}
		var cmd tea.Cmd
		a.colonInput, cmd = a.colonInput.Update(msg)
		return a, cmd
	}

	// If search input is focused AND we're in search panel, handle insert mode
	if a.focusedPanel == panelSearch && a.search.input.Focused() {
		return a.updateSearchInput(msg)
	}

	// If visual mode is active, check for visual-specific keys first
	if a.isVisualActive() {
		if msg, ok := msg.(tea.KeyMsg); ok {
			if m, cmd, handled := a.updateVisual(msg); handled {
				return m, cmd
			}
			// Fall through to normal for navigation keys
			return a.updateNormal(msg)
		}
	}

	// Handle key messages in normal mode
	if msg, ok := msg.(tea.KeyMsg); ok {
		return a.updateNormal(msg)
	}

	// Pass spinner ticks etc.
	var cmd tea.Cmd
	a.search, cmd = a.search.Update(msg)
	return a, cmd
}
