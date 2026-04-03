package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// searchCacheTTL is how long cached search results remain valid.
// After this, the entry is ignored and the user's next search will re-fetch.
const searchCacheTTL = 30 * time.Minute

// savedSearchTrack is the JSON-serializable form of a track in the search cache.
type savedSearchTrack struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration int64  `json:"duration_ms"`
}

// savedSearchEntry is a single cached search result.
type savedSearchEntry struct {
	Query    string             `json:"query"`
	Tracks   []savedSearchTrack `json:"tracks"`
	CachedAt int64              `json:"cached_at"` // unix timestamp
}

// savedSearchCache is the on-disk format.
type savedSearchCache struct {
	Entries []savedSearchEntry `json:"entries"`
}

func searchCachePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "vimyt", "search_cache.json"), nil
}

// LoadSearchCache loads cached search results for the given query from disk.
// Returns nil if not found or expired.
func LoadSearchCache(query string) []Track {
	path, err := searchCachePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var sc savedSearchCache
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil
	}
	now := time.Now().Unix()
	for _, e := range sc.Entries {
		if e.Query == query && now-e.CachedAt < int64(searchCacheTTL.Seconds()) {
			tracks := make([]Track, 0, len(e.Tracks))
			for _, st := range e.Tracks {
				tracks = append(tracks, Track{
					ID:       st.ID,
					Title:    st.Title,
					Artist:   st.Artist,
					Duration: time.Duration(st.Duration) * time.Millisecond,
				})
			}
			return tracks
		}
	}
	return nil
}

// SaveSearchCache persists search results for a query to disk.
// Keeps only the most recent entry per query, and evicts expired entries.
func SaveSearchCache(query string, tracks []Track) {
	path, err := searchCachePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	// Load existing cache
	var sc savedSearchCache
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &sc)
	}

	// Build new entry
	now := time.Now().Unix()
	entry := savedSearchEntry{
		Query:    query,
		CachedAt: now,
	}
	for _, t := range tracks {
		entry.Tracks = append(entry.Tracks, savedSearchTrack{
			ID:       t.ID,
			Title:    t.Title,
			Artist:   t.Artist,
			Duration: t.Duration.Milliseconds(),
		})
	}

	// Replace existing entry for this query, evict expired, cap at 20 entries
	var kept []savedSearchEntry
	for _, e := range sc.Entries {
		if e.Query == query {
			continue // will be replaced
		}
		if now-e.CachedAt >= int64(searchCacheTTL.Seconds()) {
			continue // expired
		}
		kept = append(kept, e)
	}
	kept = append(kept, entry)
	if len(kept) > 20 {
		kept = kept[len(kept)-20:]
	}
	sc.Entries = kept

	data, err := json.Marshal(sc)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
