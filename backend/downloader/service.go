package downloader

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"strings"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) CreateDownload(req DownloadPayload) (*Task, error) {
	if req.Mode != "download" {
		return nil, errors.New("this method only handles download")
	}

	source, err := s.GetSource(req)
	if err != nil {
		log.Fatalln("[Downloader] Failed to get the source", err)
		return nil, err
	}
	if source == SourceYoutube {
		_, err := s.downloadFromYoutube(req)
		if err != nil {
			return nil, err
		}

	}
	log.Println("[Downloader] Pushed Download To The Queue")

	return &Task{
		ID:       0,
		Progress: 0,
		URL:      req.URL,
		Status:   "Downloading",
		Source:   source.String(),
	}, nil
}

func (s *Service) downloadFromYoutube(req DownloadPayload) (bool, error) {
	cmd := exec.Command(
		"yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"-o", "./songs/%(title)s.%(ext)s", // Added -o and removed quotes
		req.URL, // The URL is the last argument
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("CombinedOutput Failed", req.URL)
		return false, err
	}

	fmt.Printf("Current UTC time:\n%s\n", string(output))

	err = cmd.Run()
	if err != nil {
		return false, err
	}

	return true, nil
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
