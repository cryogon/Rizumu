// Package downloader : handling all the downloading of songs
package downloader

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"cryogon/rizumu-backend/store"
	"cryogon/rizumu-backend/utils"

	"github.com/bogem/id3v2"
)

// --- Regex Definitions for Log Parsing ---
var (
	ytProgressRegex     = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)
	spotdlTotalRegex    = regexp.MustCompile(`Found (\d+) songs in .*`)
	spotdlDownloadRegex = regexp.MustCompile(`^(Downloaded|INFO:spotdl.download.downloader:Downloaded)`)
	spotdlErrorRegex    = regexp.MustCompile(`^(LookupError:|ERROR:spotdl.download.progress_handler:LookupError)`)
)

type Service struct {
	JobQueue chan *Task
	Store    *store.Store
}

func NewService(db *store.Store) *Service {
	s := &Service{
		JobQueue: make(chan *Task, 100),
		Store:    db,
	}
	go s.worker()
	return s
}

func (s *Service) CreateDownload(req DownloadPayload, taskID int64) (*Task, error) {
	if taskID <= 0 {
		return nil, errors.New("invalid task ID: must be > 0")
	}

	source, err := s.GetSource(req)
	if err != nil {
		return nil, err
	}

	newTask := &Task{
		ID:     taskID,
		URL:    req.URL,
		Status: StatusPending,
		Source: source.String(),
	}

	s.JobQueue <- newTask
	log.Printf("[Downloader] Queued job for Song ID %d", taskID)
	return newTask, nil
}

func (s *Service) worker() {
	log.Println("[Worker] Ready for jobs.")

	for task := range s.JobQueue {
		log.Printf("[Worker] Processing %d (%s)", task.ID, task.Source)

		// FIX 1: Handle error for Downloading status
		if err := s.Store.UpdateSongStatus(context.Background(), task.ID, string(StatusDownloading)); err != nil {
			log.Printf("[Worker] WARN: Failed to update status to Downloading: %v", err)
		}

		finalPath, err := s.runDownloadJob(task)

		if err != nil {
			log.Printf("[Worker] ERROR job %d: %v", task.ID, err)
			// FIX 2: Handle error for Failed status
			if dbErr := s.Store.UpdateSongStatus(context.Background(), task.ID, string(StatusFailed)); dbErr != nil {
				log.Printf("[Worker] CRITICAL: Failed to mark job as failed: %v", dbErr)
			}
		} else {
			log.Printf("[Worker] FINISHED job %d. Path: %s", task.ID, finalPath)
			// FIX 3: Handle error for UpdatePath
			if dbErr := s.Store.UpdateSongPath(context.Background(), task.ID, finalPath, 0); dbErr != nil {
				log.Printf("[Worker] CRITICAL: Failed to save final path: %v", dbErr)
			}
		}
	}
}

func (s *Service) runDownloadJob(task *Task) (string, error) {
	req := DownloadPayload{URL: task.URL, Mode: "download"}
	source, _ := s.GetSource(req)

	switch source {
	case SourceYoutube:
		return s.downloadFromYoutube(task)
	case SourceYTMusic:
		return s.downloadFromYoutubeMusic(task)
	case SourceSpotify:
		return s.downloadFromSpotify(task)
	case SourceOsu:
		return s.downloadFromOsu(task)
	}

	return "", errors.New("unsupported source in worker")
}

// --- 1. YOUTUBE / YT MUSIC ---

func (s *Service) downloadFromYoutube(task *Task) (string, error) {
	finalPath := fmt.Sprintf("./songs/%d.mp3", task.ID)

	cmd := exec.Command(
		"yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"-o", finalPath,
		"--progress",
		"--newline",
		task.URL,
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	go s.streamOutput(stdoutPipe, "[yt-dlp-out]", task)
	go s.streamOutput(stderrPipe, "[yt-dlp-err]", task)

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	return finalPath, nil
}

func (s *Service) downloadFromYoutubeMusic(task *Task) (string, error) {
	return s.downloadFromYoutube(task)
}

// --- 2. SPOTIFY ---

func (s *Service) downloadFromSpotify(task *Task) (string, error) {
	finalPath := fmt.Sprintf("./songs/%d.mp3", task.ID)
	spotdlTemplate := fmt.Sprintf("./songs/%d.{output-ext}", task.ID)

	cmd := exec.Command(
		"spotdl",
		"download",
		task.URL,
		"--format", "mp3",
		"--output", spotdlTemplate,
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err == nil {
		go s.streamOutput(stdoutPipe, "[spotdl-out]", task)
		go s.streamOutput(stderrPipe, "[spotdl-err]", task)

		if err := cmd.Wait(); err == nil {
			if _, err := os.Stat(finalPath); err == nil {
				return finalPath, nil
			}
		}
	}
	log.Printf("[Worker] spotdl failed for %d. Attempting Brute Force Fallback...", task.ID)
	song, err := s.Store.GetSong(context.Background(), task.ID)
	if err != nil {
		return "", fmt.Errorf("fallback failed: could not get metadata: %w", err)
	}

	// JIT Metadata Fetching
	if song.DurationMs == 0 {
		log.Printf("[Worker] Metadata missing for %d. Fetching via spotdl save...", task.ID)

		metaPath := fmt.Sprintf("./songs/meta_%d.spotdl", task.ID)

		saveCmd := exec.Command("spotdl", "save", task.URL, "--save-file", metaPath)
		if out, err := saveCmd.CombinedOutput(); err != nil {
			log.Printf("[Worker] WARN: Metadata fetch failed: %s", string(out))
			return "", err
		} else {
			type SpotdlMeta struct {
				Name     string  `json:"name"`
				Artist   string  `json:"artist"`
				Duration float64 `json:"duration"` // Seconds
			}
			var metaList []SpotdlMeta

			jsonBytes, err := os.ReadFile(metaPath)
			if err == nil {
				if jsonErr := json.Unmarshal(jsonBytes, &metaList); jsonErr == nil && len(metaList) > 0 {
					m := metaList[0]
					song.Title = m.Name
					song.Artist = m.Artist
					song.DurationMs = int64(m.Duration * 1000)

					if dbErr := s.Store.UpdateSongFullMetadata(context.Background(), song); dbErr != nil {
						log.Printf("WARN: Failed to persist fetched metadata: %v", dbErr)
					}
				}
			}
			// Cleanup temp file
			_ = os.Remove(metaPath)
		}
	}

	searchQuery := fmt.Sprintf("ytsearch5:%s - %s", song.Artist, song.Title)
	songDuration := song.DurationMs / 1000
	lowerBound := max(songDuration-30, 0)
	matchFilter := fmt.Sprintf("duration > %d & duration < %d", lowerBound, songDuration+30) // adding extra 30 secs just in case
	log.Printf("[Worker] Searching YouTube for: %s; filter:%s", searchQuery, matchFilter)

	fallbackCmd := exec.Command(
		"yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"-o", finalPath,
		"--progress",
		"--newline",
		"--match-filter", matchFilter,
		// stop after downloading exactly one file
		"--max-downloads", "1",
		searchQuery,
	)

	stdout, _ := fallbackCmd.StdoutPipe()
	stderr, _ := fallbackCmd.StderrPipe()

	if err := fallbackCmd.Start(); err != nil {
		return "", fmt.Errorf("fallback start failed: %w", err)
	}

	go s.streamOutput(stdout, "[fallback-out]", task)
	go s.streamOutput(stderr, "[fallback-err]", task)
	if err := fallbackCmd.Wait(); err != nil {
		// Check if it's the specific "Max Downloads Reached" error (Exit Code 101)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 101 {
			// This is actually a success for us if the file exists
			if _, statErr := os.Stat(finalPath); statErr == nil {
				log.Printf("[Worker] Fallback hit max-downloads limit (expected).")
				s.applyMetadata(finalPath, task.ID)
				return finalPath, nil
			}
		}
		return "", fmt.Errorf("fallback download failed: %w", err)
	}

	log.Printf("[Worker] Fallback successful for %s", song.Title)
	s.applyMetadata(finalPath, task.ID)
	return finalPath, nil
}

func (s *Service) downloadFromOsu(task *Task) (string, error) {
	log.Printf("[Worker] Starting Osu processing for: %s", task.URL)

	parsedURL, err := url.Parse(task.URL)
	if err != nil {
		return "", err
	}

	downloadURL := task.URL
	if strings.Contains(parsedURL.Host, "osu.ppy.sh") {
		paths := strings.Split(strings.TrimPrefix(parsedURL.Path, "/"), "/")
		if len(paths) >= 2 && paths[0] == "beatmapsets" {
			beatmapsetID := paths[1]
			downloadURL = fmt.Sprintf("https://osu.direct/api/d/%s", beatmapsetID)
		} else {
			return "", fmt.Errorf("unsupported osu url format: %s", task.URL)
		}
	}

	tempOszPath := fmt.Sprintf("./songs/temp_%d.osz", task.ID)
	outFile, err := os.Create(tempOszPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	resp, err := utils.HTTPClient.Get(downloadURL)
	if err != nil {
		if closeErr := outFile.Close(); closeErr != nil {
			log.Printf("WARN: Failed to close temp file: %v", closeErr)
		}
		return "", fmt.Errorf("http get failed: %w", err)
	}
	defer func() {
		if cErr := resp.Body.Close(); cErr != nil {
			log.Printf("WARN: Failed to close response body: %v", cErr)
		}
	}()

	totalSizeStr := resp.Header.Get("Content-Length")
	totalSize, _ := strconv.ParseInt(totalSizeStr, 10, 64)

	var downloadedBytes int64 = 0
	lastReportedProgress := -5.0
	buf := make([]byte, 32*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				if closeErr := outFile.Close(); closeErr != nil {
					log.Printf("WARN: Failed to close temp file on write error: %v", closeErr)
				}
				return "", writeErr
			}

			downloadedBytes += int64(n)

			if totalSize > 0 {
				progress := (float64(downloadedBytes) / float64(totalSize)) * 100
				if (progress - lastReportedProgress) >= 1.0 {
					if dbErr := s.Store.UpdateSongProgress(context.Background(), task.ID, progress); dbErr != nil {
						log.Printf("WARN: Failed to update DB progress: %v", dbErr)
					}
					lastReportedProgress = progress
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if closeErr := outFile.Close(); closeErr != nil {
				log.Printf("WARN: Failed to close temp file on read error: %v", closeErr)
			}
			return "", readErr
		}
	}

	if err := outFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close osz file: %w", err)
	}

	r, err := zip.OpenReader(tempOszPath)
	if err != nil {
		if rmErr := os.Remove(tempOszPath); rmErr != nil {
			log.Printf("WARN: Failed to remove invalid zip: %v", rmErr)
		}
		return "", fmt.Errorf("failed to open osz: %w", err)
	}
	defer func() {
		if cErr := r.Close(); cErr != nil {
			log.Printf("WARN: Failed to close zip reader: %v", cErr)
		}
	}()

	var meta OsuMetadata
	foundOsu := false

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".osu") {
			rc, err := f.Open()
			if err == nil {
				meta = parseOsuFile(rc)
				foundOsu = true
				if cErr := rc.Close(); cErr != nil {
					log.Printf("WARN: Failed to close .osu file: %v", cErr)
				}
			} else {
				log.Printf("WARN: Failed to open .osu file: %v", err)
			}
		}
	}

	if !foundOsu {
		if rmErr := os.Remove(tempOszPath); rmErr != nil {
			log.Printf("WARN: Failed to remove temp osz: %v", rmErr)
		}
		return "", errors.New("no .osu file found in archive")
	}

	var finalAudioPath string
	var finalImagePath string

	for _, f := range r.File {
		if strings.EqualFold(f.Name, meta.AudioFilename) {
			finalAudioPath = fmt.Sprintf("./songs/%d.mp3", task.ID)
			if err := extractFileFromZip(f, finalAudioPath); err != nil {
				return "", err
			}
		}
		if meta.BgFilename != "" && strings.EqualFold(f.Name, meta.BgFilename) {
			ext := filepath.Ext(f.Name)
			finalImagePath = fmt.Sprintf("./covers/%d%s", task.ID, ext)
			if mkErr := os.MkdirAll("./covers", 0o755); mkErr != nil {
				log.Printf("WARN: Failed to create covers dir: %v", mkErr)
			}
			if extractErr := extractFileFromZip(f, finalImagePath); extractErr != nil {
				log.Printf("WARN: Failed to extract bg image: %v", extractErr)
			}
		}
	}

	if finalAudioPath == "" {
		if rmErr := os.Remove(tempOszPath); rmErr != nil {
			log.Printf("WARN: Failed to remove temp osz: %v", rmErr)
		}
		return "", errors.New("audio file not found in osz")
	}

	tag, err := id3v2.Open(finalAudioPath, id3v2.Options{Parse: true})
	if err == nil {
		defer func() {
			if cErr := tag.Close(); cErr != nil {
				log.Printf("WARN: Failed to close ID3 tag: %v", cErr)
			}
		}()

		tag.SetTitle(meta.Title)
		tag.SetArtist(meta.Artist)
		tag.SetAlbum("osu!")

		if finalImagePath != "" {
			imgBytes, err := os.ReadFile(finalImagePath)
			if err == nil {
				pic := id3v2.PictureFrame{
					Encoding:    id3v2.EncodingISO,
					MimeType:    "image/jpeg",
					PictureType: id3v2.PTFrontCover,
					Description: "Cover",
					Picture:     imgBytes,
				}
				tag.AddAttachedPicture(pic)
			} else {
				log.Printf("WARN: Failed to read cover image for embedding: %v", err)
			}
		}
		if saveErr := tag.Save(); saveErr != nil {
			log.Printf("WARN: Failed to save ID3 tags: %v", saveErr)
		}
	} else {
		log.Printf("WARN: Failed to open mp3 for tagging: %v", err)
	}

	dbUpdate := &store.Song{
		ID:       task.ID,
		FilePath: finalAudioPath,
		ImageURL: finalImagePath,
		Title:    meta.Title,
		Artist:   meta.Artist,
		BPM:      meta.BPM,
	}

	if err := s.Store.UpdateSongFullMetadata(context.Background(), dbUpdate); err != nil {
		log.Printf("WARN: Failed to update DB metadata: %v", err)
	}

	if rmErr := os.Remove(tempOszPath); rmErr != nil {
		log.Printf("WARN: Failed to cleanup temp osz: %v", rmErr)
	}

	return finalAudioPath, nil
}

// --- HELPERS ---

func extractFileFromZip(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = rc.Close()
	}()

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := outFile.Close(); cErr != nil {
			log.Printf("WARN: Failed to close extracted file %s: %v", destPath, cErr)
		}
	}()

	if _, err := io.Copy(outFile, rc); err != nil {
		return err
	}
	return nil
}

func (s *Service) GetSource(req DownloadPayload) (DownloadSource, error) {
	u, err := url.Parse(req.URL)
	if err != nil {
		return SourceUnknown, err
	}

	host := u.Host
	if strings.Contains(host, "music.youtube.com") {
		return SourceYTMusic, nil
	}
	if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be") {
		return SourceYoutube, nil
	}
	if strings.Contains(host, "spotify.com") {
		return SourceSpotify, nil
	}
	if strings.Contains(host, "osu.ppy.sh") {
		return SourceOsu, nil
	}

	return SourceUnknown, fmt.Errorf("unknown source for url: %s", req.URL)
}

func (s *Service) streamOutput(pipe io.ReadCloser, prefix string, task *Task) {
	scanner := bufio.NewScanner(pipe)

	totalSongs := 1.0
	processedSongs := 0.0

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("%s %s", prefix, line)

		// Logic A: Spotify (Multi-file parsing)
		if task.Source == "spotify" {
			if matches := spotdlTotalRegex.FindStringSubmatch(line); len(matches) > 1 {
				if total, err := strconv.ParseFloat(matches[1], 64); err == nil && total > 0 {
					totalSongs = total
				}
			}
			if spotdlDownloadRegex.MatchString(line) {
				processedSongs++
			}
			if spotdlErrorRegex.MatchString(line) {
				processedSongs++
			}

			progress := (processedSongs / totalSongs) * 100
			// Ignore DB error in logging loop
			_ = s.Store.UpdateSongProgress(context.Background(), task.ID, progress)

		} else {
			// Logic B: YouTube/Standard (Percentage Parsing)
			matches := ytProgressRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
					_ = s.Store.UpdateSongProgress(context.Background(), task.ID, p)
				}
			}
		}
	}
}

func (s *Service) applyMetadata(path string, songID int64) {
	song, err := s.Store.GetSong(context.Background(), songID)
	if err != nil {
		return
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return
	}
	defer func() { _ = tag.Close() }()

	// Scenario A: Manual Download (DB says "Unknown") -> Read from File to DB
	if strings.HasPrefix(song.Title, "Unknown") {
		newTitle := tag.Title()
		newArtist := tag.Artist()
		if newTitle != "" {
			// Update the local struct so we are up to date
			song.Title = newTitle
			song.Artist = newArtist
			s.Store.UpdateSongFullMetadata(context.Background(), song)
		}
		return
	}

	// Scenario B: Synced (DB has Data) -> Write DB to File
	// This ensures the file matches what we see in the app
	tag.SetTitle(song.Title)
	tag.SetArtist(song.Artist)
	tag.SetAlbum(song.Album)

	// Embed Cover Art
	if song.ImageURL != "" {
		var imgBytes []byte

		// Fetch from URL or Read from Disk
		if strings.HasPrefix(song.ImageURL, "http") {
			resp, err := utils.HTTPClient.Get(song.ImageURL)
			if err == nil {
				defer resp.Body.Close()
				imgBytes, _ = io.ReadAll(resp.Body)
			}
		} else {
			// Local file (e.g. from Osu extraction)
			imgBytes, _ = os.ReadFile(song.ImageURL)
		}

		if len(imgBytes) > 0 {
			tag.AddAttachedPicture(id3v2.PictureFrame{
				Encoding:    id3v2.EncodingISO,
				MimeType:    "image/jpeg",
				PictureType: id3v2.PTFrontCover,
				Description: "Cover",
				Picture:     imgBytes,
			})
		}
	}

	if err := tag.Save(); err != nil {
		log.Printf("WARN: Failed to save ID3 tags: %v", err)
	}
}
