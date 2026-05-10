package multiplayer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	mrand "math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/MrJc01/crom-jogos-bitguitar/internal/game"
	"github.com/gorilla/websocket"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client

	mu           sync.Mutex
	Players      map[string]*PlayerState
	CurrentState string // "WAITING" or "PLAYING"
	CurrentSong  *game.Song
	NextSong     *game.Song
	DurationMs   int64
	StartTimeMs  int64
	CountdownMs  int64
	AllSongs     []game.Song
}

type PlayerState struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
	Combo int    `json:"combo"`
}

type StateMessage struct {
	Type        string        `json:"type"` // "STATE" ou "LEADERBOARD"
	State       string        `json:"state,omitempty"`
	Song        *game.Song    `json:"song,omitempty"`
	NextSong    *game.Song    `json:"nextSong,omitempty"`
	DurationMs  int64         `json:"durationMs,omitempty"`
	StartTimeMs int64         `json:"startTimeMs,omitempty"`
	CountdownMs int64         `json:"countdownMs,omitempty"`
	Leaderboard []PlayerState `json:"leaderboard,omitempty"`
}

type ClientMessage struct {
	Type  string `json:"type"` // "HELLO" ou "SCORE"
	Name  string `json:"name,omitempty"`
	Score int    `json:"score,omitempty"`
	Combo int    `json:"combo,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func NewHub(songs []game.Song) *Hub {
	var next *game.Song
	if len(songs) > 0 {
		n := songs[mrand.Intn(len(songs))]
		next = &n
	}
	h := &Hub{
		broadcast:    make(chan []byte),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		clients:      make(map[*Client]bool),
		Players:      make(map[string]*PlayerState),
		AllSongs:     songs,
		CurrentState: "WAITING",
		CountdownMs:  10000,
		NextSong:     next,
	}
	go h.run()
	go h.radioDJ()
	go h.broadcaster()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.Players[client.id] = &PlayerState{Name: "Anon", Score: 0, Combo: 0}
			h.mu.Unlock()
			h.sendStateTo(client)
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.Players, client.id)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
					delete(h.Players, client.id)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) radioDJ() {
	for {
		if len(h.AllSongs) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		h.mu.Lock()
		h.CurrentState = "WAITING"
		h.CountdownMs = 15000
		h.CurrentSong = nil
		for _, p := range h.Players {
			p.Score = 0
			p.Combo = 0
		}
		h.mu.Unlock()

		h.broadcastState()
		time.Sleep(15 * time.Second)

		h.mu.Lock()
		h.CurrentState = "PLAYING"
		
		var song game.Song
		if h.NextSong != nil {
			song = *h.NextSong
		} else {
			song = h.AllSongs[mathRandInt(len(h.AllSongs))]
		}
		
		h.CurrentSong = &song
		h.StartTimeMs = time.Now().UnixMilli() + 3000
		
		// Sortear proxima
		ns := h.AllSongs[mathRandInt(len(h.AllSongs))]
		h.NextSong = &ns
		
		// Calcular duracao
		durationMs := int64(60000)
		if len(song.Beatmap) > 0 {
			durationMs = int64(song.Beatmap[len(song.Beatmap)-1].TimeMs) + 5000
		}
		h.DurationMs = durationMs
		h.mu.Unlock()

		h.broadcastState()

		time.Sleep(time.Duration(durationMs) * time.Millisecond)
		h.saveRankings(song.Title)
	}
}

func mathRandInt(n int) int {
	return mrand.Intn(n)
}

func (h *Hub) saveRankings(songTitle string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	type RankEntry struct {
		Name  string
		Score int
		Date  string
	}
	var entries []RankEntry
	for _, p := range h.Players {
		if p.Score > 0 {
			entries = append(entries, RankEntry{Name: p.Name, Score: p.Score, Date: time.Now().Format(time.RFC3339)})
		}
	}
	if len(entries) == 0 {
		return
	}
	f, err := os.OpenFile("rankings.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		b, _ := json.Marshal(map[string]interface{}{"song": songTitle, "ranks": entries})
		f.Write(append(b, '\n'))
		f.Close()
	}
}

func (h *Hub) broadcaster() {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		<-ticker.C
		h.mu.Lock()
		var board []PlayerState
		for _, p := range h.Players {
			board = append(board, *p)
		}
		h.mu.Unlock()

		msg := StateMessage{
			Type:        "LEADERBOARD",
			Leaderboard: board,
		}
		b, _ := json.Marshal(msg)
		h.broadcast <- b
	}
}

func (h *Hub) broadcastState() {
	h.mu.Lock()
	msg := StateMessage{
		Type:        "STATE",
		State:       h.CurrentState,
		Song:        h.CurrentSong,
		NextSong:    h.NextSong,
		DurationMs:  h.DurationMs,
		StartTimeMs: h.StartTimeMs,
		CountdownMs: h.CountdownMs,
	}
	h.mu.Unlock()
	b, _ := json.Marshal(msg)
	h.broadcast <- b
}

func (h *Hub) sendStateTo(c *Client) {
	h.mu.Lock()
	msg := StateMessage{
		Type:        "STATE",
		State:       h.CurrentState,
		Song:        h.CurrentSong,
		NextSong:    h.NextSong,
		DurationMs:  h.DurationMs,
		StartTimeMs: h.StartTimeMs,
		CountdownMs: h.CountdownMs,
	}
	h.mu.Unlock()
	b, _ := json.Marshal(msg)
	c.send <- b
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	idBytes := make([]byte, 4)
	rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)

	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), id: id}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	id   string
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			c.hub.mu.Lock()
			if p, ok := c.hub.Players[c.id]; ok {
				if msg.Type == "HELLO" {
					if msg.Name != "" {
						p.Name = msg.Name
					}
				} else if msg.Type == "SCORE" {
					p.Score = msg.Score
					p.Combo = msg.Combo
				}
			}
			c.hub.mu.Unlock()
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}
