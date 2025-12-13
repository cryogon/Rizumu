package player

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"time"

	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/store"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

type Player struct {
	playlists  []store.Song
	ctrl       *beep.Ctrl
	format     beep.Format
	streamer   beep.StreamSeekCloser
	songIndex  int
	store      *store.Store
	downloader *downloader.Service
	isPlaying  bool
}

func NewPlayer(downloader *downloader.Service, s *store.Store) *Player {
	return &Player{
		playlists:  []store.Song{},
		songIndex:  -1,
		store:      s,
		isPlaying:  false,
		downloader: downloader,
	}
}

func (p *Player) AddSongs(songs []store.Song) {
	p.playlists = append(p.playlists, songs...)
}

func (p *Player) AddSong(song store.Song) {
	p.playlists = append(p.playlists, song)
}

func (p *Player) loadSong(songIndex int) error {
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
		f.Close()
		return err
	}

	p.streamer = streamer
	p.format = format
	p.songIndex = songIndex

	if p.ctrl == nil {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	}

	p.ctrl = &beep.Ctrl{
		Streamer: beep.Seq(p.streamer, beep.Callback(func() {
			p.onSongEnd()
		})),
	}

	fmt.Printf("Loaded Song: %s\n", song.Title)

	return nil
}

func (p *Player) Play() {
	if p.songIndex == -1 {
		err := p.loadSong(0)
		if err != nil {
			log.Fatal(err)
			return
		}
	}

	p.isPlaying = true
	if p.ctrl == nil {
		p.isPlaying = false
		return
	}
	speaker.Play(p.ctrl)

	p.prepareNextSong()
}

func (p *Player) Pause() {
	if p.ctrl == nil {
		return
	}
	speaker.Lock()
	p.ctrl.Paused = true
	p.isPlaying = false
	speaker.Unlock()
}

func (p *Player) Resume() {
	if p.ctrl == nil {
		return
	}

	speaker.Lock()
	p.ctrl.Paused = false
	p.isPlaying = true
	speaker.Unlock()
}

func (p *Player) TogglePause() string {
	if p.ctrl == nil {
		return "No Song Playing"
	}

	speaker.Lock()
	p.ctrl.Paused = !p.ctrl.Paused
	p.isPlaying = !p.ctrl.Paused
	speaker.Unlock()

	if p.ctrl.Paused {
		return "Paused"
	}
	return "Resumed"
}

func (p *Player) Seek(position int) error {
	if p.streamer == nil {
		return fmt.Errorf("no song loaded")
	}
	speaker.Lock()
	defer speaker.Unlock()

	return p.streamer.Seek(position)
}

func (p *Player) Position() int {
	if p.streamer == nil {
		return 0
	}
	return p.streamer.Position()
}

func (p *Player) PositionInSeconds() int {
	speaker.Lock()
	positionSamples := p.Position()
	positionDuration := p.format.SampleRate.D(positionSamples)
	speaker.Unlock()
	return int(positionDuration.Seconds())
}

func (p *Player) Next() error {
	nextIndex := p.getNextSongIndex()

	speaker.Clear()

	err := p.loadSong(nextIndex)
	if err != nil {
		return err
	}

	p.Play()
	return nil
}

func (p *Player) Previous() error {
	prevIndex := p.songIndex - 1
	if prevIndex < 0 {
		prevIndex = len(p.playlists) - 1
	}
	speaker.Clear()

	err := p.loadSong(prevIndex)
	if err != nil {
		return err
	}

	p.Play()
	return nil
}

func (p *Player) Stop() {
	p.isPlaying = false
	speaker.Clear()
}

func (p *Player) Close() {
	if p.streamer == nil {
		return
	}

	p.Stop()
	p.streamer.Close()
	p.playlists = []store.Song{}
}

func (p *Player) IsPlaying() bool {
	return p.isPlaying
}

func (p *Player) CurrentSong() store.Song {
	return p.playlists[p.songIndex]
}

func (p *Player) onSongEnd() {
	go func() {
		err := p.Next()
		if err != nil {
			fmt.Printf("Error playing next song")
		}
	}()
}

func (p *Player) getNextSongIndex() int {
	nextIndex := p.songIndex + 1
	if nextIndex >= len(p.playlists) {
		nextIndex = 0
	}
	return nextIndex
}

func (p *Player) prepareNextSong() {
	nextIndex := p.getNextSongIndex()

	song := p.playlists[nextIndex]

	if song.FilePath != "" {
		return
	}

	// Remove all consecutive "Not Available" songs at nextIndex
	for nextIndex < len(p.playlists) {
		song = p.playlists[nextIndex]

		if song.Status != "Not Available" {
			break
		}

		fmt.Printf("Removing unavailable song: %v\n", song)
		p.playlists = slices.Delete(p.playlists, nextIndex, nextIndex+1)

		if nextIndex <= p.songIndex && p.songIndex > 0 {
			p.songIndex--
		}
	}

	// Check if we still have songs and if the next song already has a file
	if nextIndex >= len(p.playlists) || p.playlists[nextIndex].FilePath != "" {
		return
	}
	song = p.playlists[nextIndex]

	go func() {
		fmt.Printf("Downloading Next Song: Song: %v", song)
		err := p.downloader.DownloadSong(song)
		if err != nil {
			fmt.Printf("Failed downloading next song: Song: %v. err: %v", song, err)
			return
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		errCount := 0
		maxErrors := 5

		for range ticker.C {
			if errCount >= maxErrors {
				fmt.Print("Max Errors reached while checking downloading")
				break
			}

			s, err := p.store.GetSong(context.Background(), song.ID)
			if err != nil {
				errCount++
				continue
			}

			if s.Status == "Not Available" {
				fmt.Printf("Song became unavailable during download: %v\n", s)
				// Remove this song and try preparing the next one
				p.removeSongAndPrepareNext(nextIndex)
				return
			}

			if s.Status == "Downloaded" {
				fmt.Printf("Downlaoded Song: %v", s)
				p.playlists[nextIndex] = *s
				break
			}
		}
	}()
}

func (p *Player) removeSongAndPrepareNext(index int) {
	if index >= len(p.playlists) {
		return
	}

	fmt.Printf("Removing bad song at index %d\n", index)
	p.playlists = slices.Delete(p.playlists, index, index+1)

	// Adjust songIndex if we deleted something before current position
	if index <= p.songIndex && p.songIndex > 0 {
		p.songIndex--
	}

	// Try to prepare the next song again
	p.prepareNextSong()
}
