package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type FileMetadata struct {
	Title      string
	Artist     string
	Album      string
	DurationMs int64
	Bitrate    int
	Format     string
	Size       int64
}

type ffprobeOutput struct {
	Format struct {
		Filename   string `json:"filename"`
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
		Tags       struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
			Album  string `json:"album"`
		} `json:"tags"`
	} `json:"format"`
}

// ProbeFile analyzes a media file using ffprobe
func ProbeFile(path string) (*FileMetadata, error) {
	// Check file existence
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Run ffprobe
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var data ffprobeOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Parse fields
	meta := &FileMetadata{
		Size: info.Size(), // Use OS reported size for accuracy
	}

	// Duration (seconds string -> ms int64)
	if durSec, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil {
		meta.DurationMs = int64(durSec * 1000)
		fmt.Printf("Duration: Sec(%f) Ms (%f)", durSec, meta.DurationMs)

	}

	// Bitrate (bps string -> kbps int)
	if br, err := strconv.ParseInt(data.Format.BitRate, 10, 64); err == nil {
		meta.Bitrate = int(br / 1000)
	}

	// Format
	meta.Format = data.Format.FormatName

	// Tags
	meta.Title = data.Format.Tags.Title
	meta.Artist = data.Format.Tags.Artist
	meta.Album = data.Format.Tags.Album

	// Fallback: If tags are empty, try to guess from filename
	if meta.Title == "" {
		filename := filepath.Base(path)
		ext := filepath.Ext(filename)
		rawName := strings.TrimSuffix(filename, ext)
		// Simple heuristic: "Artist - Title"
		parts := strings.SplitN(rawName, " - ", 2)
		if len(parts) == 2 {
			if meta.Artist == "" {
				meta.Artist = parts[0]
			}
			meta.Title = parts[1]
		} else {
			meta.Title = rawName
		}
	}

	return meta, nil
}
