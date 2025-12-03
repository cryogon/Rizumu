use color_eyre::Result;
use crossterm::event::{self, Event, KeyCode, KeyEventKind};
use ratatui::{
    DefaultTerminal, Frame,
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    widgets::{Block, Cell, List, ListItem, ListState, Row, Table, TableState},
};
use serde::Deserialize;
use std::time::Duration;
use tokio::sync::mpsc;

#[derive(Deserialize)]
struct CategoryItem {
    id: i64,
    name: String,
    image_url: String,
    user_id: i64,
    description: String,
    source_type: String,
    created_at: String,
    song_count: i64,
}

#[derive(Deserialize)]
struct Song {
    id: String,
    title: String,
    artist: String,
    duration: String,
}

#[derive(Debug, Clone, PartialEq)]
enum Focus {
    Tab,
    Browser,
    Content,
}

struct App {
    focus: Focus,

    categories: Vec<String>,
    items: Vec<CategoryItem>,
    songs: Vec<Song>,

    cat_state: ListState,
    item_state: ListState,
    song_state: TableState,

    is_loading: bool,
}

enum AppEvent {
    Input(crossterm::event::KeyEvent),
    BrowserLoaded(Result<Vec<CategoryItem>>),
    SongsLoaded(Result<Vec<Song>>),
    Tick,
}

impl App {
    fn new() -> Self {
        let mut cat_state = ListState::default();
        cat_state.select(Some(0));

        Self {
            focus: Focus::Browser,
            categories: vec![
                "Playlists".into(),
                "Artists".into(),
                "Albums".into(),
                "Provider".into(),
            ],
            items: vec![],
            songs: vec![],
            is_loading: false,
            cat_state,
            item_state: ListState::default(),
            song_state: TableState::default(),
        }
    }

    fn move_selection(&mut self, delta: i32) {
        let get_new_index = |current: Option<usize>, len: usize, delta: i32| -> usize {
            let i = current.unwrap_or(0);
            let new_i = i as i32 + delta;
            if new_i < 0 {
                len - 1
            } else if new_i >= len as i32 {
                0
            } else {
                new_i as usize
            }
        };

        match self.focus {
            Focus::Tab => {
                let len = self.categories.len();
                if len == 0 {
                    return;
                }
                let new_idx = get_new_index(self.cat_state.selected(), len, delta);
                self.cat_state.select(Some(new_idx));
            }
            Focus::Browser => {
                let len = self.items.len();
                if len == 0 {
                    return;
                }
                let new_idx = get_new_index(self.item_state.selected(), len, delta);
                self.item_state.select(Some(new_idx));
            }
            Focus::Content => {
                let len = self.songs.len();
                if len == 0 {
                    return;
                }
                let new_idx = get_new_index(self.song_state.selected(), len, delta);
                self.song_state.select(Some(new_idx));
            }
        }
    }
}

async fn fetch_items(category: String) -> Result<Vec<CategoryItem>> {
    match category.as_str() {
        // "Playlists" => {}
        _ => {
            let playlists = reqwest::get("http://localhost:8080/playlists")
                .await?
                .json::<Vec<CategoryItem>>()
                .await?;
            Ok(playlists)
        }
    }
}

async fn fetch_songs(item_id: i64) -> Result<Vec<Song>> {
    let songs = reqwest::get("http://localhost:8080/songs")
        .await?
        .json::<Vec<Song>>()
        .await?;
    Ok(songs)
}

#[tokio::main]
async fn main() -> Result<()> {
    color_eyre::install()?;
    let terminal = ratatui::init();

    // Create channel for Async -> UI communication
    let (tx, rx) = mpsc::channel(10);

    // Spawn Tick/Input loop
    let tx_inp = tx.clone();
    tokio::spawn(async move {
        loop {
            if event::poll(Duration::from_millis(250)).unwrap() {
                if let Event::Key(key) = event::read().unwrap() {
                    if key.kind == KeyEventKind::Press {
                        tx_inp.send(AppEvent::Input(key)).await.unwrap();
                    }
                }
            } else {
                tx_inp.send(AppEvent::Tick).await.unwrap();
            }
        }
    });

    let app = App::new();
    let result = run(terminal, app, rx, tx).await;

    ratatui::restore();
    result
}

async fn run(
    mut terminal: DefaultTerminal,
    mut app: App,
    mut rx: mpsc::Receiver<AppEvent>,
    tx: mpsc::Sender<AppEvent>,
) -> Result<()> {
    // Initial fetch
    let tx_init = tx.clone();
    tokio::spawn(async move {
        let items = fetch_items("Playlists".to_string()).await;
        tx_init.send(AppEvent::BrowserLoaded(items)).await.unwrap();
    });

    loop {
        terminal.draw(|f| render(f, &mut app))?;

        if let Some(event) = rx.recv().await {
            match event {
                AppEvent::Input(key) => match key.code {
                    KeyCode::Char('q') => break,
                    KeyCode::Down | KeyCode::Char('j') => app.move_selection(1),
                    KeyCode::Up | KeyCode::Char('k') => app.move_selection(-1),

                    // Focus Switching
                    KeyCode::Right | KeyCode::Char('l') => {
                        app.focus = match app.focus {
                            Focus::Tab => Focus::Browser,
                            Focus::Browser => Focus::Content,
                            Focus::Content => Focus::Content,
                        }
                    }
                    KeyCode::Left | KeyCode::Char('h') => {
                        app.focus = match app.focus {
                            Focus::Tab => Focus::Tab,
                            Focus::Browser => Focus::Tab,
                            Focus::Content => Focus::Browser,
                        }
                    }

                    // Selection (ID Logic Here)
                    KeyCode::Enter => {
                        let tx_api = tx.clone();
                        match app.focus {
                            Focus::Tab => {
                                if let Some(i) = app.cat_state.selected() {
                                    let cat = app.categories[i].clone();
                                    app.is_loading = true;
                                    app.focus = Focus::Browser;
                                    tokio::spawn(async move {
                                        let res = fetch_items(cat).await;
                                        tx_api.send(AppEvent::BrowserLoaded(res)).await.unwrap();
                                    });
                                }
                            }
                            Focus::Browser => {
                                if let Some(i) = app.item_state.selected() {
                                    // CRITICAL: We get the ID from the struct
                                    if let Some(item) = app.items.get(i) {
                                        let item_id = item.id.clone();
                                        app.is_loading = true;
                                        app.focus = Focus::Content;
                                        tokio::spawn(async move {
                                            // Pass ID to API
                                            let res = fetch_songs(item_id).await;
                                            tx_api.send(AppEvent::SongsLoaded(res)).await.unwrap();
                                        });
                                    }
                                }
                            }
                            _ => {}
                        }
                    }
                    _ => {}
                },
                AppEvent::BrowserLoaded(data) => match data {
                    Ok(d) => {
                        app.items = d;
                        app.item_state.select(Some(0));
                        app.is_loading = false;
                    }
                    Err(err) => {
                        println!("Failed to load the playlists, {}", err)
                    }
                },
                AppEvent::SongsLoaded(data) => match data {
                    Ok(d) => {
                        app.songs = d;
                        app.song_state.select(Some(0));
                        app.is_loading = false;
                    }
                    Err(err) => {
                        println!("Failed to load the playlists, {}", err)
                    }
                },
                AppEvent::Tick => {}
            }
        }
    }
    Ok(())
}

fn render(frame: &mut Frame, app: &mut App) {
    let outer_layout = Layout::default()
        .direction(Direction::Horizontal)
        .constraints(vec![Constraint::Percentage(30), Constraint::Percentage(70)])
        .split(frame.area());

    let left_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints(vec![Constraint::Fill(1), Constraint::Percentage(80)])
        .split(outer_layout[0]);

    // Helper for dynamic borders based on Focus
    let get_border = |f: Focus, title: &str| {
        let color = if app.focus == f {
            Color::Yellow
        } else {
            Color::White
        };
        Block::bordered()
            .title(title.to_string())
            .border_style(Style::default().fg(color))
            .border_type(ratatui::widgets::BorderType::Rounded)
    };

    let tab_items: Vec<ListItem> = app
        .categories
        .iter()
        .map(|s| ListItem::new(s.as_str()))
        .collect();
    let tab_list = List::new(tab_items)
        .block(get_border(Focus::Tab, "Library"))
        .highlight_symbol(">> ")
        .highlight_style(Style::default().fg(Color::Cyan));

    let browser_items: Vec<ListItem> = app
        .items
        .iter()
        .map(|item| ListItem::new(item.name.clone())) // Display Name
        .collect();

    let browser_widget = List::new(browser_items)
        .block(get_border(Focus::Browser, "Browser"))
        .highlight_symbol(">> ")
        .highlight_style(Style::default().fg(Color::Cyan));

    let header_style = Style::default()
        .fg(Color::Yellow)
        .add_modifier(Modifier::BOLD);
    let selected_style = Style::default()
        .bg(Color::Blue)
        .add_modifier(Modifier::BOLD);

    let rows: Vec<Row> = app
        .songs
        .iter()
        .map(|song| {
            Row::new(vec![
                Cell::from(song.title.clone()),
                Cell::from(song.artist.clone()),
                Cell::from(song.duration.clone()),
            ])
        })
        .collect();

    let widths = [
        Constraint::Percentage(50),
        Constraint::Percentage(30),
        Constraint::Percentage(20),
    ];

    let songs_table = Table::new(rows, widths)
        .header(
            Row::new(vec!["Title", "Artist", "Length"])
                .style(header_style)
                .bottom_margin(1),
        )
        .block(get_border(Focus::Content, "Songs"))
        .row_highlight_style(selected_style)
        .highlight_symbol(">> ");
    frame.render_stateful_widget(tab_list, left_layout[0], &mut app.cat_state);
    frame.render_stateful_widget(browser_widget, left_layout[1], &mut app.item_state);
    frame.render_stateful_widget(songs_table, outer_layout[1], &mut app.song_state);
}
