// Package downloader : handling all the downloading of songs
package downloader

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Service struct {
	JobQueue chan *Task
	tasks    map[int64]*Task
	mu       sync.Mutex
	nextID   int64
}

func NewService() *Service {
	jobQueue := make(chan *Task, 20)
	s := &Service{
		JobQueue: jobQueue,
		tasks:    make(map[int64]*Task),
		mu:       sync.Mutex{},
		nextID:   1,
	}

	go s.worker()

	return s
}

func (s *Service) CreateDownload(req DownloadPayload) (*Task, error) {
	if req.Mode != "download" {
		return nil, errors.New("this method only handles download")
	}

	source, err := s.GetSource(req)
	if err != nil {
		log.Println("[Downloader] Failed to get the source", err)
		return nil, err
	}

	s.mu.Lock()
	newTask := &Task{
		ID:       s.nextID,
		Progress: 0,
		URL:      req.URL,
		Status:   StatusPending,
		Source:   source.String(),
	}
	s.nextID++
	s.tasks[newTask.ID] = newTask
	s.mu.Unlock()

	s.JobQueue <- newTask

	log.Println("[Downloader] Pushed Download To The Queue")

	return newTask, nil
}

func (s *Service) GetTaskStatus(id int64) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]

	if !ok {
		return nil, errors.New("task not found")
	}

	return task, nil
}

func (s *Service) downloadFromYoutube(task *Task) error {
	cmd := exec.Command(
		"yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"-o", "./songs/%(title)s.%(ext)s",
		"--progress",
		"--newline",
		task.URL,
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// 2. Start the command (this is non-blocking)
	if err := cmd.Start(); err != nil {
		return err
	}

	// 3. Start goroutines to stream the output
	go s.streamOutput(stdoutPipe, "[yt-dlp-out]", task)
	go s.streamOutput(stderrPipe, "[yt-dlp-err]", task)

	return cmd.Wait()
}

func (s *Service) downloadFromYoutubeMusic(task *Task) error {
	parsedURL, err := url.Parse(task.URL)
	if err != nil {
		return err
	}

	queryParams := parsedURL.Query()

	if videoID, exists := queryParams["v"]; !exists || len(videoID) == 0 {
		return errors.New("video id not found in url")
	}

	videoID := queryParams["v"][0]

	ytURL := fmt.Sprintf("https://youtube.com/watch?v=%s", videoID)
	ytTask := &Task{
		ID:  task.ID,
		URL: ytURL,
	}
	return s.downloadFromYoutube(ytTask)
}

func (s *Service) downloadFromSpotify(task *Task) error {
	outputTemplate := "./songs/{title}.{output-ext}"
	cmd := exec.Command(
		"spotdl",
		"download",
		task.URL,
		"--output", outputTemplate,
	)

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	go s.streamOutput(stdoutPipe, "[spotdl-out]", task)
	go s.streamOutput(stderrPipe, "[spotdl-err]", task)

	return cmd.Wait()
}

func (s *Service) GetSource(req DownloadPayload) (DownloadSource, error) {
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return SourceUnknown, fmt.Errorf("could not parse URL: %w", err)
	}

	host := parsedURL.Hostname()

	if strings.HasSuffix(host, "music.youtube.com") {
		return SourceYTMusic, nil
	}

	if strings.HasSuffix(host, "youtube.com") {
		return SourceYoutube, nil
	}

	if strings.HasSuffix(host, "spotify.com") {
		return SourceSpotify, nil
	}

	if strings.HasSuffix(host, "osu.ppy.sh") {
		return SourceOsu, nil
	}

	return SourceOsu, errors.New("unsupported source")
}

func (s *Service) worker() {
	log.Println("[Worker] Starting up. Ready for jobs.")

	// This "for range" loop will block and wait until
	// something new appears on the JobQueue.
	for task := range s.JobQueue {
		log.Printf("[Worker] Picked up job: %d", task.ID)

		s.updateTaskStatus(task.ID, StatusDownloading, 0, "")

		// blocking but it's running in its OWN goroutine
		err := s.runDownloadJob(task)

		if err != nil {
			log.Printf("[Worker] ERROR job %d: %v", task.ID, err)
			s.updateTaskStatus(task.ID, StatusFailed, 0, err.Error())
		} else {
			log.Printf("[Worker] FINISHED job %d", task.ID)
			s.updateTaskStatus(task.ID, StatusComplete, 100, "")
		}
	}
}

// This is the new function your worker calls.
func (s *Service) runDownloadJob(task *Task) error {
	req := DownloadPayload{URL: task.URL, Mode: "download"}
	source, err := s.GetSource(req)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	if source == SourceYoutube {
		return s.downloadFromYoutube(task)
	}

	if source == SourceYTMusic {
		return s.downloadFromYoutubeMusic(task)
	}

	if source == SourceSpotify {
		return s.downloadFromSpotify(task)
	}

	// TODO: Add case for Osu
	log.Printf("[Worker] No handler for source: %s", source.String())
	return nil
}

var (
	// This regex will find the percentage in "  [download]  10.5% of..."
	ytProgressRegex = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)

	// This one finds "Found 38 songs in..."
	spotdlTotalRegex = regexp.MustCompile(`Found (\d+) songs in .*`)

	// This one finds 'Downloaded "..."'
	spotdlDownloadRegex = regexp.MustCompile(`^(Downloaded|INFO:spotdl.download.downloader:Downloaded)`)

	// This one finds 'LookupError: ...'
	spotdlErrorRegex = regexp.MustCompile(`^(LookupError:|ERROR:spotdl.download.progress_handler:LookupError)`)
)

func (s *Service) streamOutput(pipe io.ReadCloser, prefix string, task *Task) {
	scanner := bufio.NewScanner(pipe)
	source, err := s.GetSource(DownloadPayload{URL: task.URL, Mode: "download"})
	if err != nil {
		return
	}

	totalSongs := 1.0
	processedSongs := 0.0

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("%s: %s", prefix, line)

		if source == SourceYoutube || source == SourceYTMusic {
			matches := ytProgressRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				// we found a percentage
				progress, err := strconv.ParseFloat(matches[1], 64)
				if err == nil {
					s.updateTaskStatus(task.ID, StatusDownloading, progress, "")
				}
			}
		} else if source == SourceSpotify {
			// 1. Check for the "total" line
			if matches := spotdlTotalRegex.FindStringSubmatch(line); len(matches) > 1 {
				total, err := strconv.ParseFloat(matches[1], 64)
				if err == nil && total > 0 {
					totalSongs = total
				}
			}

			// 2. Check for a "Downloaded" line
			if spotdlDownloadRegex.MatchString(line) {
				processedSongs++
			}

			// 3. Check for an "Error" line (still counts as processed)
			if spotdlErrorRegex.MatchString(line) {
				processedSongs++
			}

			// 4. Calculate and update status
			// (We do this on any matching line, so it updates)
			progress := (processedSongs / totalSongs) * 100
			s.updateTaskStatus(task.ID, StatusDownloading, progress, "")
		}
	}
}

func (s *Service) updateTaskStatus(id int64, status TaskStatus, progress float64, errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return // Task was somehow deleted?
	}

	task.Status = status
	if progress > task.Progress { // Only update if progress > current
		task.Progress = progress
	}
	if errorMsg != "" {
		task.Error = errorMsg
	}
}
