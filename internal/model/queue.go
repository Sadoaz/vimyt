package model

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// Queue manages an ordered list of tracks for playback.
type Queue struct {
	Tracks   []Track
	Current  int          // index of currently playing track, -1 if none
	Selected map[int]bool // set of selected indices (for visual select mode)
}

// savedTrack is the JSON-serializable form of a track for queue persistence.
type savedTrack struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration int64  `json:"duration_ms"` // milliseconds
}

// savedQueue is the JSON-serializable form of the queue.
type savedQueue struct {
	Tracks  []savedTrack `json:"tracks"`
	Current int          `json:"current"`
}

// NewQueue creates an empty queue.
func NewQueue() *Queue {
	return &Queue{
		Current:  -1,
		Selected: make(map[int]bool),
	}
}

// Add appends tracks to the end of the queue.
func (q *Queue) Add(tracks ...Track) {
	q.Tracks = append(q.Tracks, tracks...)
}

// Remove removes the track at index i.
func (q *Queue) Remove(i int) {
	if i < 0 || i >= len(q.Tracks) {
		return
	}
	q.Tracks = append(q.Tracks[:i], q.Tracks[i+1:]...)
	// adjust current pointer
	if q.Current == i {
		// The currently playing track was removed — no track in queue is playing
		q.Current = -1
	} else if q.Current > i {
		q.Current--
	}
	// clean up selection
	delete(q.Selected, i)
	newSel := make(map[int]bool)
	for idx := range q.Selected {
		if idx > i {
			newSel[idx-1] = true
		} else {
			newSel[idx] = true
		}
	}
	q.Selected = newSel
}

// MoveUp swaps track at index i with the one above it.
func (q *Queue) MoveUp(i int) {
	if i <= 0 || i >= len(q.Tracks) {
		return
	}
	q.Tracks[i], q.Tracks[i-1] = q.Tracks[i-1], q.Tracks[i]
	switch q.Current {
	case i:
		q.Current--
	case i - 1:
		q.Current++
	}
}

// MoveDown swaps track at index i with the one below it.
func (q *Queue) MoveDown(i int) {
	if i < 0 || i >= len(q.Tracks)-1 {
		return
	}
	q.Tracks[i], q.Tracks[i+1] = q.Tracks[i+1], q.Tracks[i]
	switch q.Current {
	case i:
		q.Current++
	case i + 1:
		q.Current--
	}
}

// Clear removes all tracks and resets state.
func (q *Queue) Clear() {
	q.Tracks = nil
	q.Current = -1
	q.Selected = make(map[int]bool)
}

// Next advances to the next track. Returns the track or nil if at end.
func (q *Queue) Next() *Track {
	if len(q.Tracks) == 0 {
		return nil
	}
	next := q.Current + 1
	if next >= len(q.Tracks) {
		return nil
	}
	q.Current = next
	return &q.Tracks[q.Current]
}

// Previous goes to the previous track. Returns the track or nil if at start.
func (q *Queue) Previous() *Track {
	if len(q.Tracks) == 0 {
		return nil
	}
	prev := q.Current - 1
	if prev < 0 {
		return nil
	}
	q.Current = prev
	return &q.Tracks[q.Current]
}

// CurrentTrack returns the currently playing track or nil.
func (q *Queue) CurrentTrack() *Track {
	if q.Current < 0 || q.Current >= len(q.Tracks) {
		return nil
	}
	return &q.Tracks[q.Current]
}

// Len returns the number of tracks in the queue.
func (q *Queue) Len() int {
	return len(q.Tracks)
}

// ToggleSelect toggles selection state of the given index.
func (q *Queue) ToggleSelect(i int) {
	if i < 0 || i >= len(q.Tracks) {
		return
	}
	if q.Selected[i] {
		delete(q.Selected, i)
	} else {
		q.Selected[i] = true
	}
}

// ClearSelection clears all selections.
func (q *Queue) ClearSelection() {
	q.Selected = make(map[int]bool)
}

// Shuffle randomizes the order of tracks in the queue.
// The currently playing track (if any) stays at its position and
// everything else is shuffled around it.
func (q *Queue) Shuffle() {
	n := len(q.Tracks)
	if n < 2 {
		return
	}
	// If a track is currently playing, move it to index 0 so
	// it stays at a known position while we shuffle the rest.
	if q.Current >= 0 && q.Current < n {
		q.Tracks[0], q.Tracks[q.Current] = q.Tracks[q.Current], q.Tracks[0]
		// Shuffle everything after position 0.
		rand.Shuffle(n-1, func(i, j int) {
			q.Tracks[i+1], q.Tracks[j+1] = q.Tracks[j+1], q.Tracks[i+1]
		})
		q.Current = 0
	} else {
		rand.Shuffle(n, func(i, j int) {
			q.Tracks[i], q.Tracks[j] = q.Tracks[j], q.Tracks[i]
		})
	}
	q.Selected = make(map[int]bool)
}

// InsertAfter inserts tracks after the given index.
func (q *Queue) InsertAfter(i int, tracks ...Track) {
	if len(tracks) == 0 {
		return
	}
	insertIdx := i + 1
	insertIdx = min(insertIdx, len(q.Tracks))
	insertIdx = max(insertIdx, 0)
	newTracks := make([]Track, 0, len(q.Tracks)+len(tracks))
	newTracks = append(newTracks, q.Tracks[:insertIdx]...)
	newTracks = append(newTracks, tracks...)
	newTracks = append(newTracks, q.Tracks[insertIdx:]...)
	// Adjust current pointer
	if q.Current >= insertIdx {
		q.Current += len(tracks)
	}
	q.Tracks = newTracks
}

// SelectedTracks returns the tracks at the selected indices in order.
func (q *Queue) SelectedTracks() []Track {
	var tracks []Track
	for i := range len(q.Tracks) {
		if q.Selected[i] {
			tracks = append(tracks, q.Tracks[i])
		}
	}
	return tracks
}

func queuePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "vimyt", "queue.json"), nil
}

// SaveQueue persists the queue tracks and current index to disk.
// StreamURLs are NOT saved (they expire).
func SaveQueue(q *Queue) error {
	path, err := queuePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	sq := savedQueue{Current: q.Current}
	for _, t := range q.Tracks {
		sq.Tracks = append(sq.Tracks, savedTrack{
			ID:       t.ID,
			Title:    t.Title,
			Artist:   t.Artist,
			Duration: t.Duration.Milliseconds(),
		})
	}
	data, err := json.MarshalIndent(sq, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadQueue restores the queue from disk. Returns a new empty queue if not found.
func LoadQueue() *Queue {
	path, err := queuePath()
	if err != nil {
		return NewQueue()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return NewQueue()
	}
	var sq savedQueue
	if err := json.Unmarshal(data, &sq); err != nil {
		return NewQueue()
	}
	q := NewQueue()
	for _, st := range sq.Tracks {
		q.Tracks = append(q.Tracks, Track{
			ID:       st.ID,
			Title:    st.Title,
			Artist:   st.Artist,
			Duration: time.Duration(st.Duration) * time.Millisecond,
		})
	}
	q.Current = sq.Current
	q.Current = min(q.Current, len(q.Tracks)-1)
	return q
}
