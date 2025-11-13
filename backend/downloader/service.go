package downloader

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os/exec"
	"strings"
)

type Service struct {
	JobQueue chan DownloadPayload
}

func NewService() *Service {
	jobQueue := make(chan DownloadPayload, 100)
	s := &Service{
		JobQueue: jobQueue,
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

	s.JobQueue <- req

	log.Println("[Downloader] Pushed Download To The Queue")

	// 3. Immediately return a "pending" task to the user.
	return &Task{
		ID:       0,
		Progress: 0,
		URL:      req.URL,
		Status:   "Pending",
		Source:   source.String(),
	}, nil
}

func (s *Service) downloadFromYoutube(req DownloadPayload) (bool, error) {
	cmd := exec.Command(
		"yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"-o", "./songs/%(title)s.%(ext)s",
		req.URL,
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return false, err
	}

	// 2. Start the command (this is non-blocking)
	if err := cmd.Start(); err != nil {
		return false, err
	}

	// 3. Start goroutines to stream the output
	go streamOutput(stdoutPipe, "[yt-dlp-out]")
	go streamOutput(stderrPipe, "[yt-dlp-err]")

	if err := cmd.Wait(); err != nil {
		return false, err
	}

	return true, nil
}

func (s *Service) downloadFromYoutubeMusic(req DownloadPayload) (bool, error) {
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return false, err
	}

	queryParams := parsedURL.Query()

	if videoID, exists := queryParams["v"]; !exists || len(videoID) == 0 {
		return false, errors.New("video id not found in url")
	}

	videoID := queryParams["v"][0]

	ytURL := fmt.Sprintf("https://youtube.com/watch?v=%s", videoID)

	return s.downloadFromYoutube(DownloadPayload{
		URL:  ytURL,
		Mode: req.Mode,
	})
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
	for req := range s.JobQueue {
		log.Printf("[Worker] Picked up job: %s", req.URL)

		// This is a blocking call, but it's okay
		// because it's running in its OWN goroutine
		// and not blocking the HTTP server.
		err := s.runDownloadJob(req)
		if err != nil {
			log.Printf("[Worker] ERROR running job %s: %v", req.URL, err)
		} else {
			log.Printf("[Worker] FINISHED job: %s", req.URL)
		}
	}
}

// This is the new function your worker calls.
func (s *Service) runDownloadJob(req DownloadPayload) error {
	source, err := s.GetSource(req)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	if source == SourceYoutube {
		_, err := s.downloadFromYoutube(req)
		return err // This will be streamed
	}

	if source == SourceYTMusic {
		_, err := s.downloadFromYoutubeMusic(req)
		return err // This will also be streamed
	}

	// TODO: Add cases for Spotify, Osu
	log.Printf("[Worker] No handler for source: %s", source.String())
	return nil
}

func streamOutput(pipe io.ReadCloser, prefix string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		log.Printf("%s: %s", prefix, scanner.Text())
	}
}
