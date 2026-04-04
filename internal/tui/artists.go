package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Sadoaz/vimyt/internal/model"
	"github.com/Sadoaz/vimyt/internal/youtube"
)

var (
	artistDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	artistNormalStyle = lipgloss.NewStyle().Padding(0, 2)
	artistCurStyle    = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("238"))
)

// updateArtistsFilter handles key events when artists filter input is active.
func (a App) updateArtistsFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "enter":
		a.artistsFilter = a.artistsFilterInp.Value()
		a.artistsFiltering = false
		a.artistsFilterInp.Blur()
		a.artistsPanelCur = 0
		a.artistsPanelScrl = 0
		return a, nil
	case msg.String() == "esc":
		if a.artistsFilterInp.Value() != "" {
			a.artistsFilterInp.SetValue("")
			a.artistsFilter = ""
			a.artistsPanelCur = 0
			a.artistsPanelScrl = 0
		} else {
			a.artistsFiltering = false
			a.artistsFilterInp.Blur()
		}
		return a, nil
	default:
		var cmd tea.Cmd
		a.artistsFilterInp, cmd = a.artistsFilterInp.Update(msg)
		a.artistsFilter = a.artistsFilterInp.Value()
		a.artistsPanelCur = 0
		a.artistsPanelScrl = 0
		return a, cmd
	}
}

// artistsFilteredList returns the filtered item count for the current level.
func (a App) artistsFilteredCount() int {
	filter := strings.ToLower(a.artistsFilter)
	switch a.artistsLevel {
	case 0:
		if filter == "" {
			return a.artistStore.Len()
		}
		count := 0
		for _, ar := range a.artistStore.Artists {
			if strings.Contains(strings.ToLower(ar.Name), filter) {
				count++
			}
		}
		return count
	case 1:
		if filter == "" {
			return len(a.artistsPanelAlbs)
		}
		count := 0
		for _, alb := range a.artistsPanelAlbs {
			if strings.Contains(strings.ToLower(alb.Title), filter) {
				count++
			}
		}
		return count
	case 2:
		if filter == "" {
			return len(a.artistsPanelTrks)
		}
		count := 0
		for _, t := range a.artistsPanelTrks {
			if strings.Contains(strings.ToLower(t.Title), filter) ||
				strings.Contains(strings.ToLower(t.Artist), filter) {
				count++
			}
		}
		return count
	}
	return 0
}

// artistsFilteredArtists returns filtered artist indices.
func (a App) artistsFilteredArtists() []*model.SavedArtist {
	filter := strings.ToLower(a.artistsFilter)
	if filter == "" {
		return a.artistStore.Artists
	}
	var result []*model.SavedArtist
	for _, ar := range a.artistStore.Artists {
		if strings.Contains(strings.ToLower(ar.Name), filter) {
			result = append(result, ar)
		}
	}
	return result
}

// artistsFilteredAlbums returns filtered albums.
func (a App) artistsFilteredAlbums() []youtube.Album {
	filter := strings.ToLower(a.artistsFilter)
	if filter == "" {
		return a.artistsPanelAlbs
	}
	var result []youtube.Album
	for _, alb := range a.artistsPanelAlbs {
		if strings.Contains(strings.ToLower(alb.Title), filter) {
			result = append(result, alb)
		}
	}
	return result
}

// artistsFilteredTracks returns filtered tracks.
func (a App) artistsFilteredTracks() []model.Track {
	filter := strings.ToLower(a.artistsFilter)
	if filter == "" {
		return a.artistsPanelTrks
	}
	var result []model.Track
	for _, t := range a.artistsPanelTrks {
		if strings.Contains(strings.ToLower(t.Title), filter) ||
			strings.Contains(strings.ToLower(t.Artist), filter) {
			result = append(result, t)
		}
	}
	return result
}

// artistsPanelMaxIdx returns the max cursor index for the current level.
func (a App) artistsPanelMaxIdx() int {
	switch a.artistsLevel {
	case 0:
		return len(a.artistsFilteredArtists()) - 1
	case 1:
		return len(a.artistsFilteredAlbums()) - 1
	case 2:
		return len(a.artistsFilteredTracks()) - 1
	}
	return 0
}

// artistPanelAlbumsMsg carries album results for the artists panel.
type artistPanelAlbumsMsg struct {
	artistIdx int
	channelID string
	albums    []youtube.Album
	err       error
}

// artistsPanelEnter handles Enter/l on the artists panel.
func (a *App) artistsPanelEnter() tea.Cmd {
	if a.artistsLevel == 0 {
		// Artist list → show albums (from cache or fetch)
		if a.artistStore.Len() == 0 || a.artistsPanelCur >= a.artistStore.Len() {
			return nil
		}
		if a.artistsPanelLoad {
			return nil
		}
		artist := a.artistStore.Artists[a.artistsPanelCur]

		// If albums are cached, show them instantly
		if len(artist.Albums) > 0 {
			a.artistsLevel = 1
			a.artistsPanelName = artist.Name
			// Convert saved albums to youtube.Album for display
			a.artistsPanelAlbs = savedToYTAlbums(artist.Albums)
			a.artistsPanelCur = 0
			a.artistsPanelScrl = 0
			return nil
		}

		// Fetch albums from YouTube
		a.artistsPanelLoad = true
		a.artistsPanelName = artist.Name
		idx := a.artistsPanelCur
		cachedChannelID := artist.ChannelID
		cmd := a.setStatus(fmt.Sprintf("Loading albums for %s...", artist.Name))
		fetchCmd := func() tea.Msg {
			if cachedChannelID != "" {
				// Use cached channel ID — skip search
				albums, err := youtube.FetchArtistAlbumsByChannel(cachedChannelID)
				return artistPanelAlbumsMsg{artistIdx: idx, channelID: cachedChannelID, albums: albums, err: err}
			}
			channelID, _, albums, err := youtube.FetchArtistAlbums(artist.Name)
			return artistPanelAlbumsMsg{artistIdx: idx, channelID: channelID, albums: albums, err: err}
		}
		return tea.Batch(cmd, fetchCmd)
	}

	if a.artistsLevel == 1 {
		// Album list → show tracks (from cache or fetch)
		if len(a.artistsPanelAlbs) == 0 || a.artistsPanelCur >= len(a.artistsPanelAlbs) {
			return nil
		}
		if a.artistsPanelLoad {
			return nil
		}
		album := a.artistsPanelAlbs[a.artistsPanelCur]

		// Check if tracks are cached in the artist store
		artistIdx := a.artistStoreIdxByName(a.artistsPanelName)
		if artistIdx >= 0 {
			for _, sa := range a.artistStore.Artists[artistIdx].Albums {
				if sa.ID == album.ID && len(sa.Tracks) > 0 {
					// Cached — show instantly
					tracks := savedAlbumTracksToModel(sa.Tracks)
					if len(tracks) == 1 {
						t := tracks[0]
						a.saveQueueUndo()
						a.qdata.Add(t)
						a.queue.cursor = a.qdata.Len() - 1
						a.qdata.Current = a.qdata.Len() - 1
						a.playTrack(&a.qdata.Tracks[a.qdata.Current], "artist")
						return nil
					}
					a.artistsLevel = 2
					a.artistsPanelTrks = tracks
					a.artistsPanelAlbN = album.Title
					a.artistsPanelCur = 0
					a.artistsPanelScrl = 0
					return nil
				}
			}
		}

		// Not cached — fetch from YouTube
		a.artistsPanelLoad = true
		a.artistsPanelAlbN = album.Title
		cmd := a.setStatus(fmt.Sprintf("Fetching \"%s\"...", album.Title))
		fetchCmd := func() tea.Msg {
			tracks, err := youtube.FetchAlbumTracks(album)
			return albumTracksMsg{album: album, tracks: tracks, err: err}
		}
		return tea.Batch(cmd, fetchCmd)
	}

	// Level 2: Track list → play the track and add to queue
	if len(a.artistsPanelTrks) == 0 || a.artistsPanelCur >= len(a.artistsPanelTrks) {
		return nil
	}
	t := a.artistsPanelTrks[a.artistsPanelCur]
	a.saveQueueUndo()
	a.qdata.Add(t)
	a.queue.cursor = a.qdata.Len() - 1
	a.qdata.Current = a.qdata.Len() - 1
	a.playTrack(&a.qdata.Tracks[a.qdata.Current], "artist")
	return nil
}

// artistsPanelBack handles h/backspace/Esc in the artists panel.
func (a *App) artistsPanelBack() {
	// Clear visual first
	if a.artistsVisual {
		a.artistsVisual = false
		return
	}
	// Clear filter first
	if a.artistsFilter != "" {
		a.artistsFilter = ""
		a.artistsFilterInp.SetValue("")
		a.artistsPanelCur = 0
		a.artistsPanelScrl = 0
		return
	}
	if a.artistsLevel == 2 {
		a.artistsLevel = 1
		for i, alb := range a.artistsPanelAlbs {
			if alb.Title == a.artistsPanelAlbN {
				a.artistsPanelCur = i
				break
			}
		}
		a.artistsPanelScrl = 0
		a.artistsPanelTrks = nil
		return
	}
	if a.artistsLevel == 1 {
		a.artistsLevel = 0
		for i, e := range a.artistStore.Artists {
			if e.Name == a.artistsPanelName {
				a.artistsPanelCur = i
				break
			}
		}
		a.artistsPanelScrl = 0
		a.artistsPanelAlbs = nil
	}
}

// handleArtistPanelAlbumsMsg processes fetched albums — caches to disk and shows them.
func (a *App) handleArtistPanelAlbumsMsg(msg artistPanelAlbumsMsg) tea.Cmd {
	a.artistsPanelLoad = false
	if msg.err != nil {
		return a.setStatus(fmt.Sprintf("Artist lookup failed: %v", msg.err))
	}

	// Cache albums to disk
	idx := msg.artistIdx
	if idx >= 0 && idx < a.artistStore.Len() {
		saved := ytToSavedAlbums(msg.albums)
		a.artistStore.SetAlbums(idx, msg.channelID, saved, time.Now().Unix())
	}

	// Show albums
	a.artistsLevel = 1
	a.artistsPanelAlbs = msg.albums
	a.artistsPanelCur = 0
	a.artistsPanelScrl = 0
	return nil
}

// artistStoreIdxByName returns the index of an artist by name, or -1.
func (a App) artistStoreIdxByName(name string) int {
	for i, ar := range a.artistStore.Artists {
		if ar.Name == name {
			return i
		}
	}
	return -1
}

// savedAlbumTracksToModel converts saved album tracks to model.Track.
func savedAlbumTracksToModel(saved []model.SavedAlbumTrack) []model.Track {
	tracks := make([]model.Track, len(saved))
	for i, s := range saved {
		tracks[i] = model.Track{
			ID:       s.ID,
			Title:    s.Title,
			Artist:   s.Artist,
			Duration: time.Duration(s.Duration) * time.Millisecond,
		}
	}
	return tracks
}

// modelTracksToSavedAlbum converts model.Track to saved album tracks for persistence.
func modelTracksToSavedAlbum(tracks []model.Track) []model.SavedAlbumTrack {
	saved := make([]model.SavedAlbumTrack, len(tracks))
	for i, t := range tracks {
		saved[i] = model.SavedAlbumTrack{
			ID:       t.ID,
			Title:    t.Title,
			Artist:   t.Artist,
			Duration: t.Duration.Milliseconds(),
		}
	}
	return saved
}

// savedToYTAlbums converts saved albums to youtube.Album for display.
func savedToYTAlbums(saved []model.SavedAlbum) []youtube.Album {
	albums := make([]youtube.Album, len(saved))
	for i, s := range saved {
		albums[i] = youtube.Album{ID: s.ID, Title: s.Title, URL: s.URL}
	}
	return albums
}

// ytToSavedAlbums converts youtube.Album to saved albums for persistence.
func ytToSavedAlbums(albums []youtube.Album) []model.SavedAlbum {
	saved := make([]model.SavedAlbum, len(albums))
	for i, a := range albums {
		saved[i] = model.SavedAlbum{ID: a.ID, Title: a.Title, URL: a.URL}
	}
	return saved
}

// handleFollowArtist adds the track's artist under cursor to the artist store.
func (a App) handleFollowArtist() (tea.Model, tea.Cmd) {
	var artist string

	switch a.focusedPanel {
	case panelSearch:
		if t := a.search.currentTrack(); t != nil {
			artist = t.Artist
		}
	case panelQueue:
		realIdx := a.queue.realIndex(a.queue.cursor)
		if a.qdata.Len() > 0 && realIdx < a.qdata.Len() {
			artist = a.qdata.Tracks[realIdx].Artist
		}
	case panelHistory:
		if t := a.history.currentTrack(); t != nil {
			artist = t.Artist
		}
	case panelPlaylist:
		if a.playlist.level == levelDetail {
			tracks := a.playlist.visibleTracks()
			if len(tracks) > 0 && a.playlist.detailCur < len(tracks) {
				artist = tracks[a.playlist.detailCur].Artist
			}
		}
	}

	if artist == "" || artist == "Unknown" {
		return a, nil
	}

	_, added := a.artistStore.Add(artist)
	if added {
		cmd := a.setStatus(fmt.Sprintf("Following artist: %s", artist))
		return a, cmd
	}
	cmd := a.setStatus(fmt.Sprintf("Already following: %s", artist))
	return a, cmd
}

// saveArtistsUndo snapshots the current artist list for undo.
func (a *App) saveArtistsUndo() {
	snap := make([]*model.SavedArtist, len(a.artistStore.Artists))
	for i, ar := range a.artistStore.Artists {
		cp := *ar
		cp.Albums = make([]model.SavedAlbum, len(ar.Albums))
		copy(cp.Albums, ar.Albums)
		snap[i] = &cp
	}
	a.artistsUndoStack = append(a.artistsUndoStack, snap)
	a.artistsRedoStack = nil // clear redo on new action
}

// artistsUndo restores the previous artist list state.
func (a *App) artistsUndo() bool {
	if len(a.artistsUndoStack) == 0 {
		return false
	}
	// Save current state to redo
	snap := make([]*model.SavedArtist, len(a.artistStore.Artists))
	for i, ar := range a.artistStore.Artists {
		cp := *ar
		cp.Albums = make([]model.SavedAlbum, len(ar.Albums))
		copy(cp.Albums, ar.Albums)
		snap[i] = &cp
	}
	a.artistsRedoStack = append(a.artistsRedoStack, snap)

	// Restore from undo stack
	prev := a.artistsUndoStack[len(a.artistsUndoStack)-1]
	a.artistsUndoStack = a.artistsUndoStack[:len(a.artistsUndoStack)-1]
	a.artistStore.Artists = prev
	_ = a.artistStore.Save()
	if a.artistsPanelCur >= a.artistStore.Len() && a.artistsPanelCur > 0 {
		a.artistsPanelCur = a.artistStore.Len() - 1
	}
	return true
}

// artistsRedo re-applies the last undone action.
func (a *App) artistsRedo() bool {
	if len(a.artistsRedoStack) == 0 {
		return false
	}
	// Save current state to undo
	snap := make([]*model.SavedArtist, len(a.artistStore.Artists))
	for i, ar := range a.artistStore.Artists {
		cp := *ar
		cp.Albums = make([]model.SavedAlbum, len(ar.Albums))
		copy(cp.Albums, ar.Albums)
		snap[i] = &cp
	}
	a.artistsUndoStack = append(a.artistsUndoStack, snap)

	next := a.artistsRedoStack[len(a.artistsRedoStack)-1]
	a.artistsRedoStack = a.artistsRedoStack[:len(a.artistsRedoStack)-1]
	a.artistStore.Artists = next
	_ = a.artistStore.Save()
	if a.artistsPanelCur >= a.artistStore.Len() && a.artistsPanelCur > 0 {
		a.artistsPanelCur = a.artistStore.Len() - 1
	}
	return true
}

// artistsPanelDelete removes the artist at cursor from the store.
func (a *App) artistsPanelDelete() {
	if a.artistsLevel != 0 {
		return
	}
	if a.artistsPanelCur < 0 || a.artistsPanelCur >= a.artistStore.Len() {
		return
	}
	a.saveArtistsUndo()
	a.artistStore.Remove(a.artistsPanelCur)
	if a.artistsPanelCur >= a.artistStore.Len() && a.artistsPanelCur > 0 {
		a.artistsPanelCur--
	}
}

// renderArtistsPanelConstrained renders the artists panel content.
func (a App) renderArtistsPanelConstrained(width, height int) string {
	focused := a.focusedPanel == panelArtists
	var b strings.Builder

	if a.artistsLevel == 0 {
		// Artist list
		filtered := a.artistsFilteredArtists()
		total := len(filtered)
		if a.artistStore.Len() == 0 {
			b.WriteString("  No artists followed.\n")
			b.WriteString(artistDimStyle.Render("  Press A on a track to follow an artist"))
			return b.String()
		}
		if total == 0 {
			b.WriteString("  No matches.")
			return b.String()
		}

		maxVisible := height
		if maxVisible < 1 {
			maxVisible = 10
		}

		if a.artistsPanelCur < a.artistsPanelScrl {
			a.artistsPanelScrl = a.artistsPanelCur
		}
		if a.artistsPanelCur >= a.artistsPanelScrl+maxVisible {
			a.artistsPanelScrl = a.artistsPanelCur - maxVisible + 1
		}
		a.artistsPanelScrl = min(a.artistsPanelScrl, total-maxVisible)
		a.artistsPanelScrl = max(a.artistsPanelScrl, 0)
		start := a.artistsPanelScrl
		end := min(start+maxVisible, total)

		// Visual selection range for level 0
		var visLo, visHi int
		if a.artistsVisual {
			visLo, visHi = a.artistsAnchor, a.artistsPanelCur
			if visLo > visHi {
				visLo, visHi = visHi, visLo
			}
		}

		for i := start; i < end; i++ {
			artist := filtered[i]
			isCursor := i == a.artistsPanelCur && focused
			isSelected := a.artistsVisual && i >= visLo && i <= visHi
			style := artistNormalStyle
			prefix := "  "
			if isCursor {
				style = artistCurStyle
				prefix = "> "
			} else if isSelected {
				style = artistCurStyle
			}

			albumInfo := ""
			if len(artist.Albums) > 0 {
				albumInfo = plCountStyle.Render(fmt.Sprintf("(%d albums)", len(artist.Albums)))
			}
			line := fmt.Sprintf("%2d  %s  %s", i+1, artist.Name, albumInfo)

			if a.artistsPanelLoad && isCursor && a.artistsPanelName == artist.Name {
				line += artistDimStyle.Render("  (loading...)")
			}

			content := prefix + line
			rendered := style.Render(content)
			if isCursor && width > 0 {
				rendered = marquee(rendered, width, a.tickCount)
			} else if width > 0 {
				rendered = ansi.Truncate(rendered, width, "")
			}
			b.WriteString(rendered)
			b.WriteString("\n")
		}
	} else if a.artistsLevel == 1 {
		// Album list
		b.WriteString(artistDimStyle.Render("  Albums") + "\n")
		if a.artistsPanelLoad {
			b.WriteString(artistDimStyle.Render(fmt.Sprintf("  Fetching \"%s\"...", a.artistsPanelAlbN)) + "\n")
		}

		filteredAlbs := a.artistsFilteredAlbums()
		if len(filteredAlbs) == 0 {
			if len(a.artistsPanelAlbs) == 0 {
				b.WriteString("  No albums found.")
			} else {
				b.WriteString("  No matches.")
			}
			return b.String()
		}

		maxVisible := height - 2
		if maxVisible < 1 {
			maxVisible = 10
		}

		if a.artistsPanelCur < a.artistsPanelScrl {
			a.artistsPanelScrl = a.artistsPanelCur
		}
		if a.artistsPanelCur >= a.artistsPanelScrl+maxVisible {
			a.artistsPanelScrl = a.artistsPanelCur - maxVisible + 1
		}
		a.artistsPanelScrl = min(a.artistsPanelScrl, len(filteredAlbs)-maxVisible)
		a.artistsPanelScrl = max(a.artistsPanelScrl, 0)
		start := a.artistsPanelScrl
		end := min(start+maxVisible, len(filteredAlbs))

		// Visual selection range for level 1
		var visLo, visHi int
		if a.artistsVisual {
			visLo, visHi = a.artistsAnchor, a.artistsPanelCur
			if visLo > visHi {
				visLo, visHi = visHi, visLo
			}
		}

		for i := start; i < end; i++ {
			album := filteredAlbs[i]
			isCursor := i == a.artistsPanelCur && focused
			isSelected := a.artistsVisual && i >= visLo && i <= visHi
			style := artistNormalStyle
			prefix := "  "
			if isCursor {
				style = artistCurStyle
				prefix = "> "
			} else if isSelected {
				style = artistCurStyle
			}

			// Show track count if cached
			trackInfo := ""
			artistIdx := a.artistStoreIdxByName(a.artistsPanelName)
			if artistIdx >= 0 {
				for _, sa := range a.artistStore.Artists[artistIdx].Albums {
					if sa.ID == album.ID && len(sa.Tracks) > 0 {
						trackInfo = plCountStyle.Render(fmt.Sprintf("(%d tracks)", len(sa.Tracks)))
						break
					}
				}
			}
			line := fmt.Sprintf("%2d  %s  %s", i+1, album.Title, trackInfo)

			if a.artistsPanelLoad && isCursor {
				line += artistDimStyle.Render("  (loading...)")
			}

			content := prefix + line
			rendered := style.Render(content)
			if isCursor && width > 0 {
				rendered = marquee(rendered, width, a.tickCount)
			} else if width > 0 {
				rendered = ansi.Truncate(rendered, width, "")
			}
			b.WriteString(rendered)
			b.WriteString("\n")
		}
	}

	if a.artistsLevel == 2 {
		// Track list
		b.WriteString(artistDimStyle.Render("  Tracks") + "\n")

		filteredTrks := a.artistsFilteredTracks()
		if len(filteredTrks) == 0 {
			if len(a.artistsPanelTrks) == 0 {
				b.WriteString("  No tracks found.")
			} else {
				b.WriteString("  No matches.")
			}
			return b.String()
		}

		maxVisible := height - 2
		if maxVisible < 1 {
			maxVisible = 10
		}

		if a.artistsPanelCur < a.artistsPanelScrl {
			a.artistsPanelScrl = a.artistsPanelCur
		}
		if a.artistsPanelCur >= a.artistsPanelScrl+maxVisible {
			a.artistsPanelScrl = a.artistsPanelCur - maxVisible + 1
		}
		a.artistsPanelScrl = min(a.artistsPanelScrl, len(filteredTrks)-maxVisible)
		a.artistsPanelScrl = max(a.artistsPanelScrl, 0)
		start := a.artistsPanelScrl
		end := min(start+maxVisible, len(filteredTrks))

		// Visual selection range
		var visLo, visHi int
		if a.artistsVisual {
			visLo, visHi = a.artistsAnchor, a.artistsPanelCur
			if visLo > visHi {
				visLo, visHi = visHi, visLo
			}
		}

		favSet := a.playlist.store.FavoritesSet()

		for i := start; i < end; i++ {
			t := filteredTrks[i]
			isCursor := i == a.artistsPanelCur && focused
			isSelected := a.artistsVisual && i >= visLo && i <= visHi
			style := artistNormalStyle
			prefix := "  "
			if isCursor {
				style = artistCurStyle
				prefix = "> "
			} else if isSelected {
				style = artistCurStyle
			}

			heart := ""
			if favSet[t.ID] {
				heart = " <3"
			}

			dur := formatDuration(t.Duration)
			hasBackground := isCursor || isSelected
			var line string
			if hasBackground {
				line = fmt.Sprintf("%2d  %s  %s", i+1, t.Title, dur)
			} else {
				line = fmt.Sprintf("%2d  %s  %s", i+1, t.Title, artistDimStyle.Render(dur))
				if heart != "" {
					heart = favStyle.Render(heart)
				}
			}

			content := prefix + line + heart
			rendered := style.Render(content)
			if isCursor && width > 0 {
				rendered = marquee(rendered, width, a.tickCount)
			} else if width > 0 {
				rendered = ansi.Truncate(rendered, width, "")
			}
			b.WriteString(rendered)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderArtistsCompact renders a compact summary when the panel is not focused.
func (a App) renderArtistsCompact(width int) string {
	count := a.artistStore.Len()
	line := fmt.Sprintf("  %d artists", count)
	if width > 0 {
		line = ansi.Truncate(line, width, "")
	}
	return line + "\n"
}
