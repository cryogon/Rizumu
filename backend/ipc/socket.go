package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"cryogon/rizumu-backend/player"
	"cryogon/rizumu-backend/store"
)

type IPCHandler struct {
	clients  []net.Conn
	player   *player.Player
	commands chan Command
	store    store.Store
}

func NewIPCHandler(player *player.Player, store store.Store) *IPCHandler {
	return &IPCHandler{
		clients:  make([]net.Conn, 0),
		commands: make(chan Command, 10),
		player:   player,
		store:    store,
	}
}

func (h *IPCHandler) Init() {
	socketPath := "/tmp/rizumu.sock"
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	go h.broadcastPlayerState()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		fmt.Printf("Client Joined")
		go h.handleClient(conn)
	}
}

func (h *IPCHandler) handleClient(conn net.Conn) {
	defer func() {
		err := conn.Close()
		if err != nil {
			fmt.Printf("[IPC] Failed to close client: %v", err)
		}
		h.removeClient(conn)
	}()

	h.clients = append(h.clients, conn)
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var cmd Command
		if err := json.Unmarshal(scanner.Bytes(), &cmd); err != nil {
			fmt.Printf("[IPC] Failed to parse command: %v", err)
			continue
		}
		h.handleCommands(cmd)
	}
}

func (h *IPCHandler) removeClient(conn net.Conn) {
	for i, c := range h.clients {
		if c == conn {
			h.clients = append(h.clients[:i], h.clients[i+1:]...)
			break
		}
	}
}

func (h *IPCHandler) handleCommands(cmd Command) {
	fmt.Printf("[IPC] Got Cmd %v - %s\n", cmd, CmdPlay)
	switch cmd.Type {
	case CmdPlay:
		h.player.Close()
		if cmd.PlaylistID > 0 {
			var filteredSong []store.Song
			songs, err := h.store.GetSongsByPlaylist(context.Background(), cmd.PlaylistID)
			if err != nil {
				fmt.Printf("[IPC] Failed to fetch playlists. %v", err)
				return
			}

			for _, song := range songs {
				if song.ID >= cmd.SongID {
					filteredSong = append(filteredSong, *song)
				}
			}
			h.player.AddSongs(filteredSong)
		} else {
			song, err := h.store.GetSong(context.Background(), cmd.SongID)
			if err != nil {
				fmt.Printf("[IPC] Failed to fetch song. %v", err)
				return
			}
			h.player.AddSong(*song)
		}
		h.player.Play()
	case CmdPause:
		h.player.TogglePause()
	case CmdNext:
		h.player.Next()
	case CmdPrev:
		h.player.Previous()
	}
}

func (h *IPCHandler) broadcastPlayerState() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !h.player.IsPlaying() {
			continue
		}
		song := h.player.CurrentSong()
		songPos := h.player.PositionInSeconds()

		msg := PlayerState{
			Playing:  true,
			SongID:   song.ID,
			SongName: song.Title,
			Artist:   song.Artist,
			Progress: songPos,
			Duration: int(song.DurationMs / 60),
		}

		data, err := NewMessage(msg, "player_state")
		if err != nil {
			fmt.Printf("[IPC] Failed to parse player's state. err: %v", err)
			continue
		}

		data = append(data, '\n')
		for _, client := range h.clients {
			_, err := client.Write(data)
			if err != nil {
				fmt.Printf("[IPC] Failed to broadcast player's state. err: %v", err)
				continue
			}
		}
	}
}
