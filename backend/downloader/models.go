package downloader

type DownloadPayload struct {
	URL  string `json:"url"`
	Mode string `json:"mode"` // download | stream
}

type Task struct {
	ID       int    `json:"id"`
	URL      string `json:"url"`
	Source   string `json:"source"`   // youtube | spotify | osu!
	Progress int    `json:"progress"` // download progress
	Status   string `json:"status"`   // Downloading | Completed | Failed
}

type DownloadSource int

const (
	SourceUnknown DownloadSource = iota // 0 (This is our "zero value")
	SourceYoutube                       // 1
	SourceSpotify                       // 2
	SourceOsu
	SourceYTMusic
)

func (s DownloadSource) String() string {
	switch s {
	case SourceYoutube:
		return "youtube"
	case SourceSpotify:
		return "spotify"
	case SourceYTMusic:
		return "youtube-music"
	case SourceOsu:
		return "osu!"
	default:
		return "unknown"
	}
}
