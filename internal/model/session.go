package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Session holds the state to persist between app launches.
type Session struct {
	View        int    `json:"view"`         // 0=search, 1=queue, 2=playlist
	SearchQuery string `json:"search_query"` // last search query
	SearchCur   int    `json:"search_cur"`   // cursor in search results

	QueueCur     int `json:"queue_cur"`      // cursor in queue
	HistoryCur   int `json:"history_cur"`    // cursor in history panel
	RadioHistCur int `json:"radio_hist_cur"` // cursor in radio history panel

	PLLevel     int `json:"pl_level"`      // 0=list, 1=detail
	PLListCur   int `json:"pl_list_cur"`   // cursor in playlist list
	PLDetailCur int `json:"pl_detail_cur"` // cursor in playlist detail

	Zoomed bool `json:"zoomed"` // panel zoom state

	// Playback resume state
	PlaybackPos float64 `json:"playback_pos"` // seconds into current track
	RadioActive bool    `json:"radio_active"` // was radio mode on
	RadioSeed   string  `json:"radio_seed"`   // radio seed title

	// Volume
	Volume int `json:"volume"` // playback volume 0-100

	// Settings
	Autoplay       bool   `json:"autoplay"`         // auto-advance to next track when current ends
	Shuffle        bool   `json:"shuffle"`          // randomize next track selection
	PinSearch      bool   `json:"pin_search"`       // keep search panel expanded when unfocused
	PinPlaylist    bool   `json:"pin_playlist"`     // keep playlist detail expanded when unfocused
	ShowHistory    bool   `json:"show_history"`     // show history panel below playlists
	ShowRadio      bool   `json:"show_radio"`       // show radio history panel below play history
	PinRadio       bool   `json:"pin_radio"`        // keep radio history expanded when unfocused
	RelNumbers     bool   `json:"rel_numbers"`      // show relative line numbers (vim-style)
	AutoFocusQueue bool   `json:"auto_focus_queue"` // focus queue panel when playing a track
	CookieBrowser  string `json:"cookie_browser"`   // browser for yt-dlp cookie auth (empty = off)
	ShowArtists    bool   `json:"show_artists"`     // show artists panel
	PinArtists     bool   `json:"pin_artists"`      // keep artists expanded when unfocused
	ArtistsCur     int    `json:"artists_cur"`      // cursor in artists panel
	LoopTrack      bool   `json:"loop_track"`       // loop current track on EOF
	LoopCount      int    `json:"loop_count"`       // remaining loops (0 = infinite)
	LoopTotal      int    `json:"loop_total"`       // original loop count for display
}

func sessionPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "vimyt", "session.json"), nil
}

// SessionExists returns true if a session file exists on disk.
func SessionExists() bool {
	path, err := sessionPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// LoadSession reads the session from disk. Returns zero-value Session if not found.
func LoadSession() Session {
	path, err := sessionPath()
	if err != nil {
		return Session{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}
	}
	return s
}

// SaveSession writes the session to disk.
func SaveSession(s Session) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
