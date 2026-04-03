// Package youtube provides YouTube search and audio URL resolution via yt-dlp.
package youtube

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Sadoaz/vimyt/internal/model"
)

// maxSearchCacheSize caps the number of in-memory search cache entries.
const maxSearchCacheSize = 50

// searchCache stores results per query to avoid redundant yt-dlp calls.
var (
	searchMu       sync.RWMutex
	searchCache    = make(map[string][]model.Track)
	searchCacheAge = make(map[string]int64) // insertion order counter
	searchCacheSeq int64                    // monotonic counter
)

// ytdlpResult is the JSON structure from yt-dlp --flat-playlist --dump-json.
type ytdlpResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Channel  string  `json:"channel"`
	Duration float64 `json:"duration"`
}

// Search queries YouTube via yt-dlp and returns up to 20 results.
// Results are cached per query string.
func Search(query string) ([]model.Track, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	// Check cache
	searchMu.RLock()
	if cached, ok := searchCache[query]; ok {
		searchMu.RUnlock()
		return cached, nil
	}
	searchMu.RUnlock()

	// Run yt-dlp search
	args := []string{
		fmt.Sprintf("ytsearch20:%s", query),
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
	}
	args = append(args, CookieArgs()...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("yt-dlp start error: %w", err)
	}

	var tracks []model.Track
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024) // large buffer for JSON lines

	for scanner.Scan() {
		var r ytdlpResult
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue // skip malformed lines
		}
		if r.ID == "" || r.Title == "" || r.Duration <= 0 {
			continue
		}

		artist := cleanArtist(r.Channel)
		title := cleanTitle(r.Title, artist)
		dur := time.Duration(r.Duration * float64(time.Second))

		tracks = append(tracks, model.Track{
			ID:       r.ID,
			Title:    title,
			Artist:   artist,
			Duration: dur,
		})
	}

	if err := cmd.Wait(); err != nil && len(tracks) == 0 {
		errMsg := strings.TrimSpace(stderrBuf.String())
		if errMsg != "" {
			return nil, fmt.Errorf("yt-dlp search failed: %s", errMsg)
		}
		return nil, fmt.Errorf("yt-dlp search failed: %w", err)
	}

	// Cache results (evict oldest if at capacity)
	searchMu.Lock()
	if len(searchCache) >= maxSearchCacheSize {
		var oldestKey string
		var oldestSeq int64 = searchCacheSeq + 1
		for k, seq := range searchCacheAge {
			if seq < oldestSeq {
				oldestSeq = seq
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(searchCache, oldestKey)
			delete(searchCacheAge, oldestKey)
		}
	}
	searchCacheSeq++
	searchCache[query] = tracks
	searchCacheAge[query] = searchCacheSeq
	searchMu.Unlock()

	return tracks, nil
}

// cleanArtist strips " - Topic" suffix from YouTube Music auto-generated channel names.
func cleanArtist(channel string) string {
	if channel == "" {
		return "Unknown"
	}
	return strings.TrimSuffix(channel, " - Topic")
}

// cleanTitle strips redundant artist prefix from YouTube video titles.
// e.g. "Coldplay - Yellow (Official Video)" with artist "Coldplay" → "Yellow (Official Video)"
func cleanTitle(title, artist string) string {
	if artist == "" || artist == "Unknown" {
		return title
	}
	lower := strings.ToLower(title)
	artistLower := strings.ToLower(artist)
	for _, sep := range []string{" - ", " – ", " — ", " ~ "} {
		prefix := artistLower + sep
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(title[len(prefix):])
		}
	}
	return title
}

// RadioMix fetches YouTube Music's auto-generated radio mix for a track.
// First tries the RDAMVM<videoId> playlist (YTM's algorithmic radio).
// If that yields fewer than 5 tracks, falls back to a yt-dlp search.
// Returns the seed track first, followed by related tracks.
// Results are cached per seed video ID.
func RadioMix(seed model.Track) ([]model.Track, error) {
	cacheKey := "radio:" + seed.ID

	// Check cache
	searchMu.RLock()
	if cached, ok := searchCache[cacheKey]; ok {
		searchMu.RUnlock()
		return cached, nil
	}
	searchMu.RUnlock()

	// Try RDAMVM playlist first
	pool := fetchRadioPlaylist(seed.ID)

	// Fallback: search-based radio if RDAMVM gave too few results
	if len(pool) < 5 {
		query := fmt.Sprintf("%s %s mix", seed.Artist, seed.Title)
		searched, err := Search(query)
		if err == nil {
			seen := map[string]bool{seed.ID: true}
			for _, t := range pool {
				seen[t.ID] = true
			}
			for _, t := range searched {
				if !seen[t.ID] {
					pool = append(pool, t)
					seen[t.ID] = true
				}
			}
		}
	}

	result := make([]model.Track, 0, len(pool)+1)
	result = append(result, seed)
	result = append(result, pool...)

	// Cache (evict oldest if at capacity)
	searchMu.Lock()
	if len(searchCache) >= maxSearchCacheSize {
		var oldestKey string
		var oldestSeq int64 = searchCacheSeq + 1
		for k, seq := range searchCacheAge {
			if seq < oldestSeq {
				oldestSeq = seq
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(searchCache, oldestKey)
			delete(searchCacheAge, oldestKey)
		}
	}
	searchCacheSeq++
	searchCache[cacheKey] = result
	searchCacheAge[cacheKey] = searchCacheSeq
	searchMu.Unlock()

	return result, nil
}

// fetchRadioPlaylist fetches up to 50 tracks from YTM's RDAMVM playlist.
func fetchRadioPlaylist(videoID string) []model.Track {
	url := fmt.Sprintf("https://music.youtube.com/watch?v=%s&list=RDAMVM%s", videoID, videoID)
	args := []string{
		url,
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
		"--playlist-end", "50",
	}
	args = append(args, CookieArgs()...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}

	var pool []model.Track
	seen := map[string]bool{videoID: true} // exclude seed
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		var r ytdlpResult
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		if r.ID == "" || r.Title == "" || r.Duration <= 0 || seen[r.ID] {
			continue
		}
		seen[r.ID] = true

		artist := cleanArtist(r.Channel)
		title := cleanTitle(r.Title, artist)
		dur := time.Duration(r.Duration * float64(time.Second))

		pool = append(pool, model.Track{
			ID:       r.ID,
			Title:    title,
			Artist:   artist,
			Duration: dur,
		})
	}

	_ = cmd.Wait()
	return pool
}

// ytdlpPlaylistResult extends ytdlpResult with playlist-level metadata.
type ytdlpPlaylistResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Channel  string  `json:"channel"`
	Duration float64 `json:"duration"`
	Playlist string  `json:"playlist_title"`
}

const maxImportTracks = 1000

// FetchPlaylist fetches tracks from a YouTube playlist URL (up to 500).
// Returns the playlist title (if available) and the tracks.
func FetchPlaylist(url string) (string, []model.Track, error) {
	if strings.TrimSpace(url) == "" {
		return "", nil, fmt.Errorf("empty URL")
	}

	args := []string{
		url,
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
		"--playlist-end", fmt.Sprintf("%d", maxImportTracks),
	}
	args = append(args, CookieArgs()...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, fmt.Errorf("yt-dlp pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("yt-dlp start error: %w", err)
	}

	var tracks []model.Track
	var playlistTitle string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		var r ytdlpPlaylistResult
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		if r.ID == "" || r.Title == "" || r.Duration <= 0 || seen[r.ID] {
			continue
		}
		seen[r.ID] = true

		if playlistTitle == "" && r.Playlist != "" {
			playlistTitle = r.Playlist
		}

		artist := cleanArtist(r.Channel)
		title := cleanTitle(r.Title, artist)
		dur := time.Duration(r.Duration * float64(time.Second))

		tracks = append(tracks, model.Track{
			ID:       r.ID,
			Title:    title,
			Artist:   artist,
			Duration: dur,
		})
	}

	if err := cmd.Wait(); err != nil && len(tracks) == 0 {
		errMsg := strings.TrimSpace(stderrBuf.String())
		if errMsg != "" {
			return "", nil, fmt.Errorf("yt-dlp playlist fetch failed: %s", errMsg)
		}
	}

	if len(tracks) == 0 {
		if GetCookieBrowser() == "" {
			return "", nil, fmt.Errorf("no tracks found — try enabling YT Auth in Settings for private playlists")
		}
		return "", nil, fmt.Errorf("no tracks found — check URL or try a different browser in YT Auth")
	}

	return playlistTitle, tracks, nil
}
