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
	Border(lipgloss.RoundedBorder(), true).
	Padding(0, 1)

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

type model struct {
	ipc           *IPCClient
	activeSection Section
	height        int
	width         int

	categories []string
	catCursor  int

	items      []Playlist
	itemCursor int

	songModel table.Model
	songs     []Song
}

func InitialModel(ipcClient *IPCClient) model {
	songColumns := []table.Column{
		{Title: "ID", Width: 4},
		{Title: "Title", Width: 30},
		{Title: "Artist", Width: 20},
		{Title: "Length", Width: 10},
	}

	return model{
		categories: []string{"Playlists", "Artists", "Album", "Providers"},
		catCursor:  0,
		itemCursor: 0,
		ipc:        ipcClient,
		songModel:  table.New(table.WithColumns(songColumns), table.WithFocused(true)),
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
		switch msg.Type {
		case "playlists":
			var playlists []Playlist
			if err := json.Unmarshal(msg.Data, &playlists); err == nil {
				m.items = playlists
			}
			cmds = append(cmds, listenToIPC(m.ipc))
		case "songs":
			var songs []Song
			if err := json.Unmarshal(msg.Data, &songs); err == nil {
				m.songs = songs
				var songRows []table.Row
				for _, song := range m.songs {
					songRows = append(songRows, table.Row{
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
			cmds = append(cmds, listenToIPC(m.ipc))
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		containerHeight := int(float64(m.height)*0.8) + 2
		m.songModel.SetHeight(containerHeight)
	case tea.KeyMsg:
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
			if m.activeSection == sectionItems && len(m.items) > 0 {
				selectedPlaylist := m.items[m.itemCursor]
				cmds = append(cmds, fetchSongs(m.ipc, selectedPlaylist.ID))
			}

			if m.activeSection == sectionSongs && len(m.songs) > 0 {
				playlist := m.items[m.itemCursor]
				song := m.songs[m.songModel.Cursor()]
				cmds = append(cmds, playSong(m.ipc, playlist.ID, song.ID))
			}

		case "down", "j":
			if m.activeSection == sectionCategories && m.catCursor < len(m.categories)-1 {
				m.catCursor++
			} else if m.activeSection == sectionItems && m.itemCursor < len(m.items)-1 {
				m.itemCursor++
			}
		case "up", "k":
			if m.activeSection == sectionCategories && m.catCursor > 0 {
				m.catCursor--
			} else if m.activeSection == sectionItems && m.itemCursor > 0 {
				m.itemCursor--
			}
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
	for i, item := range m.items {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(normalColor)
		if i == m.itemCursor && m.activeSection == sectionItems {
			style = style.Foreground(activeColor).Bold(true)
			cursor = ">>"
		} else if i == m.itemCursor {
			style = style.Foreground(activeColor)
		}
		itemView += fmt.Sprintf("%s %s\n", cursor, style.Render(item.Name))
	}

	leftColumnWidth := int(float64(m.width) * 0.3)
	rightColumnWidth := m.width - leftColumnWidth

	categorySectionHeight := 7
	songSectionHeight := int(float64(m.height) * 0.8)
	itemSectionHeight := songSectionHeight - categorySectionHeight

	catList := sectionStyle.
		Width(leftColumnWidth).
		Height(categorySectionHeight).
		BorderForeground(getBorderColor(sectionCategories)).
		Render(lipgloss.NewStyle().Foreground(normalColor).Bold(true).Render("LIBRARY") + "\n\n" + catView)

	itemList := sectionStyle.
		Width(leftColumnWidth).
		Height(itemSectionHeight).
		BorderForeground(getBorderColor(sectionItems)).
		Render(lipgloss.NewStyle().Foreground(normalColor).Bold(true).Render("PLAYLISTS") + "\n\n" + itemView)

	songTable := sectionStyle.
		Width(rightColumnWidth - 4).
		Height(songSectionHeight + 2).
		BorderForeground(getBorderColor(sectionSongs)).
		Render(m.songModel.View())

	leftSide := lipgloss.JoinVertical(lipgloss.Left, catList, itemList)
	topSide := lipgloss.JoinHorizontal(lipgloss.Top, leftSide, songTable)

	return topSide
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
	p := tea.NewProgram(InitialModel(ipcClient), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
