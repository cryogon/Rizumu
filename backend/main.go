package main

import (
	"fmt"

	"cryogon/rizumu-backend/downloader"
)

func main() {
	s := downloader.NewService()
	_, err := s.CreateDownload(downloader.DownloadPayload{
		Mode: "download",
		URL:  "https://www.youtube.com/watch?v=yKoBrB2dSbw",
	},
	)
	if err != nil {
		fmt.Println("GGs", err)
	}
}
