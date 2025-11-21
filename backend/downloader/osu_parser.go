package downloader

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type OsuMetadata struct {
	Title         string
	Artist        string
	AudioFilename string
	BgFilename    string
	BPM           float64
	Version       string
}

var bgEventRegex = regexp.MustCompile(`0,0,"(.+?)",`)

func parseOsuFile(r io.Reader) OsuMetadata {
	scanner := bufio.NewScanner(r)
	meta := OsuMetadata{}

	section := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Detect Section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			continue
		}

		// Parse Key:Value pairs
		if section == "[General]" {
			if strings.HasPrefix(line, "AudioFilename:") {
				meta.AudioFilename = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}

		if section == "[Metadata]" {
			if strings.HasPrefix(line, "Title:") {
				meta.Title = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
			if strings.HasPrefix(line, "Artist:") {
				meta.Artist = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
			if strings.HasPrefix(line, "Version:") {
				meta.Version = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}

		if section == "[Events]" {
			// Look for background image: 0,0,"bg.jpg"
			if bgEventRegex.MatchString(line) {
				matches := bgEventRegex.FindStringSubmatch(line)
				if len(matches) > 1 {
					meta.BgFilename = matches[1]
				}
			}
		}

		if section == "[TimingPoints]" {
			// Format: Offset, MillisecondsPerBeat, ...
			// We only take the first uninherited timing point (positive value)
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				msPerBeat, err := strconv.ParseFloat(parts[1], 64)
				if err == nil && msPerBeat > 0 {
					// BPM = 60000 / msPerBeat
					// Only capture if we haven't found BPM yet (usually the first one is main)
					if meta.BPM == 0 {
						meta.BPM = 60000 / msPerBeat
					}
				}
			}
		}
	}
	return meta
}
