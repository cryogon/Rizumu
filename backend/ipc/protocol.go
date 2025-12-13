package ipc

import "encoding/json"

type CommandType string

// What client can send
const (
	CmdPlay     CommandType = "play"
	CmdPause    CommandType = "pause"
	CmdStop     CommandType = "stop"
	CmdDownload CommandType = "download"
	CmdNext     CommandType = "next"
	CmdPrev     CommandType = "prev"
)

type Command struct {
	Type       CommandType `json:"type"`
	SongID     int64       `json:"song_id"`
	PlaylistID int64       `json:"playlist_id"`
}

type PlayerState struct {
	Playing  bool   `json:"playing"`
	SongID   int64  `json:"song_id"`
	SongName string `json:"song_name"`
	Artist   string `json:"artist"`
	Progress int    `json:"progress"` // Current Song Pos
	Duration int    `json:"duration"` // Song's Duration
}

type Message struct {
	Type string          `json:"type"` // "cmd", "pstate"
	Data json.RawMessage `json:"data"`
}

func NewMessage(cmd any, cmdType string) ([]byte, error) {
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	msg := Message{Type: cmdType, Data: data}
	return json.Marshal(msg)
}
