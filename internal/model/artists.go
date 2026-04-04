package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SavedAlbumTrack is the on-disk representation of a track in an album.
type SavedAlbumTrack struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration int64  `json:"duration_ms"`
}

// SavedAlbum is the on-disk representation of a YouTube album.
type SavedAlbum struct {
	ID     string            `json:"id"`
	Title  string            `json:"title"`
	URL    string            `json:"url"`
	Tracks []SavedAlbumTrack `json:"tracks,omitempty"` // cached tracks (fetched on-demand)
}

// SavedArtist is the on-disk representation of a followed artist.
type SavedArtist struct {
	Name      string       `json:"name"`
	ChannelID string       `json:"channel_id,omitempty"` // cached YouTube channel ID
	Albums    []SavedAlbum `json:"albums,omitempty"`     // cached albums (fetched on-demand)
	FetchedAt int64        `json:"fetched_at,omitempty"` // unix timestamp of last album fetch
}

// savedArtistStore is the JSON on-disk format.
type savedArtistStore struct {
	Artists []*SavedArtist `json:"artists"`
}

// ArtistStore manages a user-curated list of followed artists with disk persistence.
type ArtistStore struct {
	Artists []*SavedArtist
	path    string
}

func artistStorePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "vimyt", "artists.json"), nil
}

// NewArtistStore loads the artist store from disk, or creates an empty one.
func NewArtistStore() (*ArtistStore, error) {
	path, err := artistStorePath()
	if err != nil {
		return nil, err
	}
	s := &ArtistStore{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist yet — return empty store
		return s, nil
	}
	var saved savedArtistStore
	if err := json.Unmarshal(data, &saved); err != nil {
		return s, nil
	}
	s.Artists = saved.Artists
	return s, nil
}

// Save persists the artist store to disk.
func (s *ArtistStore) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	saved := savedArtistStore{Artists: s.Artists}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// Add adds a new artist to the store if not already present.
// Returns the artist (existing or new) and whether it was newly added.
func (s *ArtistStore) Add(name string) (*SavedArtist, bool) {
	nameLower := strings.ToLower(name)
	for _, a := range s.Artists {
		if strings.ToLower(a.Name) == nameLower {
			return a, false
		}
	}
	a := &SavedArtist{Name: name}
	s.Artists = append(s.Artists, a)
	_ = s.Save()
	return a, true
}

// Remove removes the artist at the given index.
func (s *ArtistStore) Remove(idx int) {
	if idx < 0 || idx >= len(s.Artists) {
		return
	}
	s.Artists = append(s.Artists[:idx], s.Artists[idx+1:]...)
	_ = s.Save()
}

// HasArtist returns true if an artist with the given name is already followed.
func (s *ArtistStore) HasArtist(name string) bool {
	nameLower := strings.ToLower(name)
	for _, a := range s.Artists {
		if strings.ToLower(a.Name) == nameLower {
			return true
		}
	}
	return false
}

// SetAlbums updates the cached albums for an artist and persists to disk.
func (s *ArtistStore) SetAlbums(idx int, channelID string, albums []SavedAlbum, fetchedAt int64) {
	if idx < 0 || idx >= len(s.Artists) {
		return
	}
	s.Artists[idx].ChannelID = channelID
	s.Artists[idx].Albums = albums
	s.Artists[idx].FetchedAt = fetchedAt
	_ = s.Save()
}

// SetAlbumTracks caches tracks for a specific album within an artist and persists to disk.
func (s *ArtistStore) SetAlbumTracks(artistIdx int, albumID string, tracks []SavedAlbumTrack) {
	if artistIdx < 0 || artistIdx >= len(s.Artists) {
		return
	}
	for i := range s.Artists[artistIdx].Albums {
		if s.Artists[artistIdx].Albums[i].ID == albumID {
			s.Artists[artistIdx].Albums[i].Tracks = tracks
			_ = s.Save()
			return
		}
	}
}

// Len returns the number of followed artists.
func (s *ArtistStore) Len() int {
	return len(s.Artists)
}
