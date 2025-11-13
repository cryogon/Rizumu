package downloader

type DownloadPayload struct {
	URL  string `json:"url"`
	Mode string `json:"mode"` // download | stream
}

type TaskStatus string

const (
	StatusPending     TaskStatus = "Pending"
	StatusDownloading TaskStatus = "Downloading"
	StatusComplete    TaskStatus = "Complete"
	StatusFailed      TaskStatus = "StatusFailed"
)

type Task struct {
	ID       int64      `json:"id"`
	URL      string     `json:"url"`
	Source   string     `json:"source"`   // youtube | spotify | osu!
	Progress float64    `json:"progress"` // download progress
	Status   TaskStatus `json:"status"`
	Error    string     `json:"error,omitempty"`
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
