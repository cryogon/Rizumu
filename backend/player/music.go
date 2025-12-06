package player

import (
	"fmt"
	"os"
	"sync"
	"time"

	"cryogon/rizumu-backend/store"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

type MusicPlayer struct {
	playlists []store.Song
	ctrl      *beep.Ctrl
	format    beep.Format
	streamer  beep.StreamSeekCloser
	songIndex int
	mu        sync.RWMutex
}

func NewMusicPlayer() *MusicPlayer {
	return &MusicPlayer{
		playlists: []store.Song{},
		songIndex: -1,
	}
}

func (p *MusicPlayer) AddSong(song store.Song) {
	p.playlists = append(p.playlists, song)
}

func (p *MusicPlayer) LoadSong(songIndex int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if songIndex < 0 || songIndex >= len(p.playlists) {
		return fmt.Errorf("invalid song index")
	}

	if p.streamer != nil {
		p.streamer.Close()
	}

	song := p.playlists[songIndex]

	f, err := os.Open(song.FilePath)
	if err != nil {
		return err
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		return nil
	}

	p.streamer = streamer
	p.format = format
	p.songIndex = songIndex

	if p.ctrl == nil {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	}

	p.ctrl = &beep.Ctrl{Streamer: beep.Loop(0, p.streamer)}
	return nil
}

func (p *MusicPlayer) Play() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.ctrl != nil {
		speaker.Play(p.ctrl)
	}
}

func (p *MusicPlayer) Pause() {
	if p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = true
		speaker.Unlock()
	}
}

func (p *MusicPlayer) Resume() {
	if p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = false
		speaker.Unlock()
	}
}

func (p *MusicPlayer) TogglePause() string {
	if p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = !p.ctrl.Paused
		speaker.Unlock()
		if p.ctrl.Paused {
			return "Paused"
		} else {
			return "Resume"
		}
	}
	return "No Song Playing"
}

func (p *MusicPlayer) Seek(position int) error {
	if p.streamer == nil {
		return fmt.Errorf("no song loaded")
	}

	speaker.Lock()
	defer speaker.Unlock()

	return p.streamer.Seek(position)
}

func (p *MusicPlayer) Position() int {
	if p.streamer == nil {
		return 0
	}
	return p.streamer.Position()
}

func (p *MusicPlayer) Next() error {
	nextIndex := p.songIndex + 1
	if nextIndex >= len(p.playlists) {
		nextIndex = 0
	}

	speaker.Clear()
	err := p.LoadSong(nextIndex)
	if err != nil {
		return err
	}
	p.Play()
	return nil
}

func (p *MusicPlayer) Previous() error {
	prevIndex := p.songIndex - 1
	if prevIndex < 0 {
		prevIndex = len(p.playlists) - 1
	}

	speaker.Clear()
	err := p.LoadSong(prevIndex)
	if err != nil {
		return err
	}
	p.Play()
	return nil
}

func (p *MusicPlayer) Close() {
	if p.streamer != nil {
		speaker.Clear()
		p.streamer.Close()
		p.playlists = []store.Song{}
	}
}

func (p *MusicPlayer) GetCurrentSong() store.Song {
	return p.playlists[p.songIndex]
}

func (p *MusicPlayer) GetPlaylist() []store.Song {
	return p.playlists
}
