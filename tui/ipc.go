package main

import (
	"encoding/json"
	"net"
	"sync"
)

type IPCClient struct {
	conn    net.Conn
	mu      sync.Mutex
	encoder *json.Encoder
	decoder *json.Decoder
}

func NewIPCClient() (*IPCClient, error) {
	c, err := net.Dial("unix", "/tmp/rizumu.sock")
	if err != nil {
		return nil, err
	}
	return &IPCClient{
		conn:    c,
		encoder: json.NewEncoder(c),
		decoder: json.NewDecoder(c),
	}, nil
}

func (c *IPCClient) Send(cmdType CommandType, PlaylistID int64, SongID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := struct {
		Type       CommandType `json:"type"`
		SongID     int64       `json:"song_id"`
		PlaylistID int64       `json:"playlist_id"`
	}{
		Type:       cmdType,
		SongID:     SongID,
		PlaylistID: PlaylistID,
	}

	return c.encoder.Encode(req)
}

func (c *IPCClient) ReadNext() (Message, error) {
	var resp Message
	err := c.decoder.Decode(&resp)
	return resp, err
}

func (c *IPCClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
