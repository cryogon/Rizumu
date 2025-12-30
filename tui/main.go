package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	normalColor = lipgloss.Color("#000000")
	activeColor = lipgloss.Color("#ff79c6")
)

var sectionStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder(), true).Padding(0).Margin(0)

type Section int

const (
	sectionCategories Section = iota
	sectionItems
	sectionSongs
)

func listenToIPC(ipc *IPCClient) tea.Cmd {
	return func() tea.Msg {
		resp, err := ipc.ReadNext()
		if err != nil {
			return err
		}
		log.Printf("Type: %s", resp.Type)
		if resp.Type == "songs" {
			log.Printf("Songs: %s", resp.Data)
		}
		return resp
	}
}

func fetchPlaylists(ipc *IPCClient) tea.Cmd {
	return func() tea.Msg {
		err := ipc.Send(CmdPlaylists, 0, 0)
		return err
	}
}

func fetchSongs(ipc *IPCClient, itemID int64) tea.Cmd {
	return func() tea.Msg {
		err := ipc.Send(CmdSongs, itemID, 0)
		return err
	}
}

func playSong(ipc *IPCClient, itemID int64, songID int64) tea.Cmd {
	return func() tea.Msg {
		err := ipc.Send(CmdPlay, itemID, songID)
		return err
	}
}

func nextSong(ipc *IPCClient) tea.Cmd {
	return func() tea.Msg {
		err := ipc.Send(CmdNext, 0, 0)
		return err
	}
}

func prevSong(ipc *IPCClient) tea.Cmd {
	return func() tea.Msg {
		err := ipc.Send(CmdPrev, 0, 0)
		return err
	}
}

func pausePlay(ipc *IPCClient) tea.Cmd {
	return func() tea.Msg {
		err := ipc.Send(CmdPause, 0, 0)
		return err
	}
}

type model struct {
	ipc           *IPCClient
	activeSection Section
	height        int
	width         int

	categories []string
	catCursor  int

	items      []Playlist
	itemCursor int
	itemOffset int

	songModel table.Model
	songs     []Song

	songProgress *ProgressBar
	playerState  PlayerState
}

func InitialModel(ipcClient *IPCClient, progressBar *ProgressBar) model {
	songColumns := []table.Column{
		{Title: "", Width: 2},
		{Title: "ID", Width: 4},
		{Title: "Title", Width: 30},
		{Title: "Artist", Width: 20},
		{Title: "Length", Width: 10},
	}

	return model{
		categories:   []string{"Playlists", "Artists", "Album", "Providers"},
		catCursor:    0,
		itemCursor:   0,
		itemOffset:   0,
		ipc:          ipcClient,
		songModel:    table.New(table.WithColumns(songColumns), table.WithFocused(true)),
		songProgress: progressBar,
	}
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, fetchPlaylists(m.ipc))
	cmds = append(cmds, listenToIPC(m.ipc))
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case Message:

		cmds = append(cmds, listenToIPC(m.ipc))
		switch msg.Type {
		case "playlists":
			var playlists []Playlist
			if err := json.Unmarshal(msg.Data, &playlists); err == nil {
				m.items = playlists
			}
		case "songs":
			var songs []Song
			if err := json.Unmarshal(msg.Data, &songs); err == nil {
				m.songs = songs
				var songRows []table.Row
				for _, song := range m.songs {
					status := ""
					if song.ID == m.playerState.SongID {
						status = "▶ "
					}
					songRows = append(songRows, table.Row{
						status,
						fmt.Sprintf("%d", song.ID),
						song.Title,
						song.Artist,
						fmt.Sprintf("%d", song.DurationMs),
					})
				}
				m.songModel.SetRows(songRows)
				m.activeSection = sectionSongs
				m.songModel.Focus()
			}
		case "player_state":
			var state PlayerState
			if err := json.Unmarshal(msg.Data, &state); err == nil {
				m.playerState = state
				// Update the progress bar component
				if state.Duration > 0 {
					m.songProgress.Update(m.width, state.Progress, state.Duration)
				}

				// Update table rows to reflect playing state
				var songRows []table.Row
				for _, song := range m.songs {
					status := ""
					if song.ID == m.playerState.SongID {
						status = "▶ "
					}
					songRows = append(songRows, table.Row{
						status,
						fmt.Sprintf("%d", song.ID),
						song.Title,
						song.Artist,
						fmt.Sprintf("%d", song.DurationMs),
					})
				}
				m.songModel.SetRows(songRows)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		availableHeight := m.height - sectionStyle.GetVerticalFrameSize()
		mainBodyHeight := availableHeight - 6
		finalRightHeight := max(mainBodyHeight-sectionStyle.GetVerticalFrameSize(), 1)

		m.songModel.SetHeight(finalRightHeight + 2)
		m.songProgress.Update(msg.Width, 0, 1)

	case tea.KeyMsg:
		availableHeight := m.height - sectionStyle.GetVerticalFrameSize()
		mainBodyHeight := availableHeight - 6
		rawTopLeftHeight := int(float64(mainBodyHeight) * 0.3)
		rawBottomLeftHeight := mainBodyHeight - rawTopLeftHeight
		finalBottomLeftHeight := rawBottomLeftHeight - sectionStyle.GetVerticalFrameSize()
		itemsPerPage := max(finalBottomLeftHeight-2, 1) // -2 for header

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.activeSection++
			if m.activeSection > 2 {
				m.activeSection = 0
			}
			if m.activeSection == sectionSongs {
				m.songModel.Focus()
			} else {
				m.songModel.Blur()
			}

		case "enter":
			switch m.activeSection {
			case sectionCategories:
				m.activeSection = sectionItems
			case sectionSongs:
				if len(m.songs) > 0 {
					playlist := m.items[m.itemCursor]
					song := m.songs[m.songModel.Cursor()]
					cmds = append(cmds, playSong(m.ipc, playlist.ID, song.ID))
				}
			case sectionItems:
				if len(m.items) > 0 {
					selectedPlaylist := m.items[m.itemCursor]
					m.activeSection = sectionSongs
					cmds = append(cmds, fetchSongs(m.ipc, selectedPlaylist.ID))
				}
			}

		case "down", "j":
			if m.activeSection == sectionCategories && m.catCursor < len(m.categories)-1 {
				m.catCursor++
			} else if m.activeSection == sectionItems && m.itemCursor < len(m.items)-1 {
				m.itemCursor++
				// Scroll down
				if m.itemCursor >= m.itemOffset+itemsPerPage {
					m.itemOffset = m.itemCursor - itemsPerPage + 1
				}
			}
		case "up", "k":
			if m.activeSection == sectionCategories && m.catCursor > 0 {
				m.catCursor--
			} else if m.activeSection == sectionItems && m.itemCursor > 0 {
				m.itemCursor--
				// Scroll up
				if m.itemCursor < m.itemOffset {
					m.itemOffset = m.itemCursor
				}
			}
		case "n":
			cmds = append(cmds, nextSong(m.ipc))
		case "p":
			cmds = append(cmds, prevSong(m.ipc))
		case " ":
			cmds = append(cmds, pausePlay(m.ipc))
		}
	}

	// Always update the table model so it can handle its own internal resizing and inputs
	var tableCmd tea.Cmd
	m.songModel, tableCmd = m.songModel.Update(msg)
	cmds = append(cmds, tableCmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// CRITICAL: Prevent crash on startup/resize when dimensions are invalid
	if m.width < 20 || m.height < 10 {
		return "Initializing..."
	}

	getBorderColor := func(section Section) lipgloss.Color {
		if m.activeSection == section {
			return activeColor
		}
		return normalColor
	}

	// Categories
	catView := ""
	for i, cat := range m.categories {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(normalColor)

		if i == m.catCursor && m.activeSection == sectionCategories {
			style = style.Foreground(activeColor).Bold(true)
			cursor = ">>"
		} else if i == m.catCursor {
			style = style.Foreground(activeColor)
		}
		catView += fmt.Sprintf("%s %s\n", cursor, style.Render(cat))
	}

	// Items
	itemView := ""

	trueWidth := m.width

	availableHeight := m.height - sectionStyle.GetVerticalFrameSize()

	footerHeight := 4
	mainBodyHeight := availableHeight - footerHeight

	rawTopLeftHeight := int(float64(mainBodyHeight) * 0.3)
	rawBottomLeftHeight := mainBodyHeight - rawTopLeftHeight

	finalTopLeftHeight := rawTopLeftHeight - sectionStyle.GetVerticalFrameSize()
	finalBottomLeftHeight := rawBottomLeftHeight - sectionStyle.GetVerticalFrameSize()
	finalRightHeight := mainBodyHeight - sectionStyle.GetVerticalFrameSize() + 1

	leftColumnWidth := int(float64(trueWidth) * 0.3)
	rightColumnWidth := (trueWidth - leftColumnWidth) - 4

	// Helpers for View
	truncate := func(s string, w int) string {
		if w <= 3 {
			return ""
		}
		if len(s) > w {
			return s[:w-3] + "..."
		}
		return s
	}

	catList := sectionStyle.
		Width(leftColumnWidth).
		Height(finalTopLeftHeight).
		BorderForeground(getBorderColor(sectionCategories)).
		Render(lipgloss.NewStyle().Foreground(normalColor).Bold(true).Render("Library") + "\n\n" + catView)

	// Calculate visible items
	itemsPerPage := finalBottomLeftHeight - 2
	if itemsPerPage < 1 {
		itemsPerPage = 1
	}

	// Slice items based on offset
	start := m.itemOffset
	end := start + itemsPerPage
	if end > len(m.items) {
		end = len(m.items)
	}
	if start > end {
		start = end // Should not happen if logic is correct
	}

	visibleItems := m.items[start:end]

	itemView = ""
	for i, item := range visibleItems {
		realIndex := start + i
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(normalColor)
		if realIndex == m.itemCursor && m.activeSection == sectionItems {
			style = style.Foreground(activeColor).Bold(true)
			cursor = ">>"
		} else if realIndex == m.itemCursor {
			style = style.Foreground(activeColor)
		}

		// Truncate name to fit width
		// Width available = leftColumnWidth - 2 (cursor) - 1 (space) = leftColumnWidth - 3
		// But let's be safe and use leftColumnWidth - 4
		displayName := truncate(item.Name, leftColumnWidth-4)

		itemView += fmt.Sprintf("%s %s\n", cursor, style.Render(displayName))
	}

	itemList := sectionStyle.
		Width(leftColumnWidth).
		Height(finalBottomLeftHeight).
		BorderForeground(getBorderColor(sectionItems)).
		Render(lipgloss.NewStyle().Foreground(normalColor).Bold(true).Render("Playlists") + "\n\n" + itemView)

	songTable := sectionStyle.
		Width(rightColumnWidth).
		Height(finalRightHeight).
		BorderForeground(getBorderColor(sectionSongs)).
		Render(m.songModel.View())

	songTitle := fmt.Sprintf(" %s - %s", m.playerState.Artist, m.playerState.SongName)
	songProgress := fmt.Sprintf("%d / %d \n", m.playerState.Progress, m.playerState.Duration)

	contentWidth := trueWidth - 2
	gapSize := contentWidth - lipgloss.Width(songTitle) - lipgloss.Width(songProgress)

	topLine := lipgloss.JoinHorizontal(
		lipgloss.Top, // Align items to the top within their own height (not relevant here, but good practice)
		songTitle,
		lipgloss.NewStyle().Width(gapSize).Render(""), // Dynamic spacing
		songProgress,
	)
	styledTopLine := lipgloss.NewStyle().Width(contentWidth).Render(topLine)
	progressBarView := m.songProgress.View()

	centeredProgressBar := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render(progressBarView)

	playerBlockContent := lipgloss.JoinVertical(
		lipgloss.Center,
		styledTopLine,
		centeredProgressBar,
	)

	playerSection := sectionStyle.
		Width(trueWidth-2).
		Height(footerHeight-sectionStyle.GetVerticalFrameSize()).
		BorderForeground(normalColor).
		Align(lipgloss.Center, lipgloss.Bottom).
		Render(playerBlockContent)

	leftSide := lipgloss.JoinVertical(lipgloss.Top, catList, itemList)
	topSide := lipgloss.JoinHorizontal(lipgloss.Top, leftSide, songTable)
	player := lipgloss.JoinVertical(lipgloss.Bottom, topSide, playerSection)

	return player
}

func main() {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	ipcClient, err := NewIPCClient()
	if err != nil {
		panic(err)
	}

	progressBar := NewProgressBar()

	p := tea.NewProgram(InitialModel(ipcClient, progressBar), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
