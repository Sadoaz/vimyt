package youtube

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Sadoaz/vimyt/internal/model"
)

// Album represents a YouTube Music album (OLAK playlist).
type Album struct {
	ID    string // playlist ID (e.g. OLAK5uy_...)
	Title string
	URL   string
}

// ytdlpChannelResult is used to extract channel_id from a search result.
type ytdlpChannelResult struct {
	ChannelID string `json:"channel_id"`
	Channel   string `json:"channel"`
}

// ytdlpReleaseResult is used to extract albums from a channel's /releases tab.
type ytdlpReleaseResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"_type"`
}

// FetchArtistAlbums finds an artist's YouTube channel by name, then fetches
// their album/release list from the channel's /releases tab.
// Returns the channel ID, resolved artist name (from YouTube), and list of albums.
func FetchArtistAlbums(artistName string) (string, string, []Album, error) {
	if strings.TrimSpace(artistName) == "" {
		return "", "", nil, fmt.Errorf("empty artist name")
	}

	// Step 1: Search for a song by this artist to find their channel ID
	channelID, resolvedName, err := findChannelID(artistName)
	if err != nil {
		return "", "", nil, err
	}

	// Step 2: Fetch releases from the channel
	albums, err := fetchReleases(channelID)
	if err != nil {
		return channelID, resolvedName, nil, err
	}

	return channelID, resolvedName, albums, nil
}

// FetchArtistAlbumsByChannel fetches albums directly from a known channel ID.
// Skips the channel lookup step when we have a cached channel ID.
func FetchArtistAlbumsByChannel(channelID string) ([]Album, error) {
	return fetchReleases(channelID)
}

// findChannelID searches YouTube for the artist and extracts the channel ID
// from the first result that matches the artist name.
func findChannelID(artistName string) (string, string, error) {
	query := fmt.Sprintf("ytsearch5:%s", artistName)
	args := []string{
		query,
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
	}
	args = append(args, CookieArgs()...)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("yt-dlp pipe error: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("yt-dlp start error: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	artistLower := strings.ToLower(artistName)

	for scanner.Scan() {
		var r ytdlpChannelResult
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		if r.ChannelID == "" {
			continue
		}
		// Prefer results where the channel name matches the artist
		channelLower := strings.ToLower(r.Channel)
		if channelLower == artistLower ||
			strings.Contains(channelLower, artistLower) ||
			strings.Contains(artistLower, channelLower) {
			_ = cmd.Wait()
			return r.ChannelID, r.Channel, nil
		}
	}

	_ = cmd.Wait()
	return "", "", fmt.Errorf("could not find channel for artist: %s", artistName)
}

// fetchReleases fetches the album list from a YouTube channel's /releases tab.
func fetchReleases(channelID string) ([]Album, error) {
	url := fmt.Sprintf("https://www.youtube.com/channel/%s/releases", channelID)
	args := []string{
		url,
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
		"--playlist-end", "30",
	}
	args = append(args, CookieArgs()...)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

	var albums []Album
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		var r ytdlpReleaseResult
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		if r.ID == "" || r.Title == "" || seen[r.ID] {
			continue
		}
		seen[r.ID] = true

		albumURL := r.URL
		if albumURL == "" {
			albumURL = fmt.Sprintf("https://www.youtube.com/playlist?list=%s", r.ID)
		}

		albums = append(albums, Album{
			ID:    r.ID,
			Title: r.Title,
			URL:   albumURL,
		})
	}

	_ = cmd.Wait()

	if len(albums) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	return albums, nil
}

// FetchAlbumTracks fetches all tracks from an album (OLAK playlist).
// Returns the album title and tracks.
func FetchAlbumTracks(album Album) ([]model.Track, error) {
	_, tracks, err := FetchPlaylist(album.URL)
	return tracks, err
}
