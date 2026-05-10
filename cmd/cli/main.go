package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/crom/bitguitar/internal/game"
	"github.com/crom/bitguitar/internal/multiplayer"
	"github.com/gdamore/tcell/v2"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gorilla/websocket"
)

// Estado global
var (
	wsConn    *websocket.Conn
	score     = 0
	combo     = 0
	myName    = ""
	gameState = "WAITING"
	board     []multiplayer.PlayerState
	boardMu   sync.Mutex

	currentSong  *game.Song
	nextSong     *game.Song
	startTimeMs  int64
	countdownMs  int64
	durationMs   int64
	wantsToPlay  = false
	isPlaying    = false

	screen tcell.Screen

	// Teclas configuráveis
	gameKeys = [4]rune{'d', 'f', 'j', 'k'}
)

type KeyConfig struct {
	Keys [4]string `json:"keys"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bitguitar_config.json")
}

func loadKeyConfig() {
	f, err := os.ReadFile(configPath())
	if err != nil {
		return
	}
	var cfg KeyConfig
	if json.Unmarshal(f, &cfg) == nil && len(cfg.Keys) == 4 {
		for i, k := range cfg.Keys {
			if len(k) > 0 {
				gameKeys[i] = rune(k[0])
			}
		}
	}
}

func saveKeyConfig() {
	cfg := KeyConfig{}
	for i, k := range gameKeys {
		cfg.Keys[i] = string(k)
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath(), b, 0644)
}

func main() {
	loadKeyConfig()

	myName = os.Getenv("USER")
	if myName == "" {
		myName = "TerminalHacker"
	}

	// Conectar WS
	u := "ws://localhost:8080/ws"
	var err error
	wsConn, _, err = websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		fmt.Println("Erro ao conectar no servidor WS na porta 8080. O monitor.sh está rodando?")
		os.Exit(1)
	}
	defer wsConn.Close()

	// Enviar HELLO como espectador
	sendHello("Spectator")

	// Receber mensagens
	go func() {
		for {
			_, msg, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			var sm multiplayer.StateMessage
			json.Unmarshal(msg, &sm)

			if sm.Type == "STATE" {
				gameState = sm.State
				currentSong = sm.Song
				nextSong = sm.NextSong
				startTimeMs = sm.StartTimeMs
				countdownMs = sm.CountdownMs
				durationMs = sm.DurationMs
				if gameState == "WAITING" {
					score = 0
					combo = 0
				}
			} else if sm.Type == "LEADERBOARD" {
				boardMu.Lock()
				board = sm.Leaderboard
				sort.Slice(board, func(i, j int) bool {
					return board[i].Score > board[j].Score
				})
				boardMu.Unlock()
			}
		}
	}()

	// Enviar Score periodicamente
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			if wantsToPlay && isPlaying {
				update := multiplayer.ClientMessage{Type: "SCORE", Score: score, Combo: combo}
				out, _ := json.Marshal(update)
				wsConn.WriteMessage(websocket.TextMessage, out)
			}
		}
	}()

	// Init UI
	screen, err = tcell.NewScreen()
	if err != nil {
		log.Fatalf("%+v", err)
	}
	if err := screen.Init(); err != nil {
		log.Fatalf("%+v", err)
	}
	defer screen.Fini()

	defStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGreen)
	screen.SetStyle(defStyle)
	screen.Clear()

	evCh := make(chan tcell.Event)
	go func() {
		for {
			evCh <- screen.PollEvent()
		}
	}()

	speakerInit := false

	// ===== LOOP PRINCIPAL =====
	for {
		if gameState == "WAITING" {
			isPlaying = false
			drawLobby(screen, defStyle)

			time.Sleep(100 * time.Millisecond)
			countdownMs -= 100
			if countdownMs < 0 {
				countdownMs = 0
			}

			select {
			case ev := <-evCh:
				if checkExit(ev) {
					return
				}
				switch ev := ev.(type) {
				case *tcell.EventKey:
					if ev.Key() == tcell.KeyRune {
						switch ev.Rune() {
						case 'j', 'J':
							if !wantsToPlay {
								wantsToPlay = true
								myName = os.Getenv("USER")
								if myName == "" {
									myName = "TerminalHacker"
								}
								sendHello(myName)
							}
						case 'c', 'C':
							showConfigScreen(screen, evCh, defStyle)
						}
					}
				case *tcell.EventResize:
					screen.Sync()
				}
			default:
			}

		} else if gameState == "PLAYING" && currentSong != nil {
			if !wantsToPlay {
				// Modo espectador
				drawSpectatorView(screen, defStyle)
				time.Sleep(100 * time.Millisecond)
				select {
				case ev := <-evCh:
					if checkExit(ev) {
						return
					}
					switch ev := ev.(type) {
					case *tcell.EventKey:
						if ev.Key() == tcell.KeyRune && (ev.Rune() == 'j' || ev.Rune() == 'J') {
							wantsToPlay = true
							myName = os.Getenv("USER")
							if myName == "" {
								myName = "TerminalHacker"
							}
							sendHello(myName)
						}
					case *tcell.EventResize:
						screen.Sync()
					}
				default:
				}
				continue
			}

			// === MODO JOGO ===
			isPlaying = true
			audioResp, err := http.Get("http://localhost:8080" + currentSong.URL)
			if err != nil {
				continue
			}
			streamer, format, err := mp3.Decode(audioResp.Body)
			if err != nil {
				continue
			}

			if !speakerInit {
				speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
				speakerInit = true
			}

			now := time.Now().UnixMilli()
			diffMs := startTimeMs - now

			if diffMs > 0 {
				for diffMs > 0 && gameState == "PLAYING" {
					screen.Clear()
					w, h := screen.Size()
					msg := fmt.Sprintf("Sincronizando Áudio... (%dms)", diffMs)
					drawText(screen, w/2-len(msg)/2, h/2, msg, defStyle)
					screen.Show()
					time.Sleep(50 * time.Millisecond)
					now = time.Now().UnixMilli()
					diffMs = startTimeMs - now
					select {
					case ev := <-evCh:
						if checkExit(ev) {
							return
						}
					default:
					}
				}
			}

			ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
			speaker.Play(ctrl)

			notes := currentSong.Beatmap
			type NoteState struct {
				Hit    bool
				Passed bool
			}
			noteStates := make([]NoteState, len(notes))
			keyMap := buildKeyMap()

			for gameState == "PLAYING" {
				screen.Clear()
				w, h := screen.Size()
				trackStartX := (w / 2) - 10
				hitY := h - 5

				currentTimeMs := int(time.Now().UnixMilli() - startTimeMs)

				// HUD
				elapsed := formatTimeMs(int64(currentTimeMs))
				total := formatTimeMs(durationMs)
				drawText(screen, 2, 0, fmt.Sprintf("SCORE: %06d | COMBO: %d | [%s]", score, combo, currentSong.Title), defStyle)
				drawText(screen, 2, 1, fmt.Sprintf("TEMPO: %s / %s", elapsed, total), defStyle.Foreground(tcell.ColorTeal))

				if nextSong != nil {
					drawText(screen, 2, 2, fmt.Sprintf("Próxima: %s", nextSong.Title), defStyle.Foreground(tcell.ColorYellow))
				}

				// Leaderboard
				drawLeaderboard(screen, trackStartX+25, 1, defStyle)

				// Pistas
				for i := 0; i < 4; i++ {
					x := trackStartX + (i * 5)
					for y := 4; y < h; y++ {
						screen.SetContent(x, y, '|', nil, defStyle)
					}
					screen.SetContent(x, hitY, rune(gameKeys[i]-32), nil, defStyle.Foreground(tcell.ColorWhite).Background(tcell.ColorGreen))
				}

				// Linha de hit
				for x := trackStartX - 2; x <= trackStartX+17; x++ {
					if x != trackStartX && x != trackStartX+5 && x != trackStartX+10 && x != trackStartX+15 {
						screen.SetContent(x, hitY, '=', nil, defStyle)
					}
				}

				// Notas
				for i, note := range notes {
					timeDiff := note.TimeMs - currentTimeMs
					y := hitY - (timeDiff / 50)

					if y > 3 && y < h {
						char := '0'
						if note.Type == 1 {
							char = '1'
						}

						style := defStyle.Foreground(tcell.ColorLightGreen).Bold(true)
						if noteStates[i].Hit {
							style = defStyle.Foreground(tcell.ColorDarkGreen)
						}

						screen.SetContent(trackStartX+(note.Lane*5), y, char, nil, style)
					}

					if timeDiff < -200 && !noteStates[i].Hit && !noteStates[i].Passed {
						noteStates[i].Passed = true
						combo = 0
					}
				}

				// Rodapé
				footer := "BitGuitar — crom.run"
				drawText(screen, w/2-len(footer)/2, h-1, footer, defStyle.Foreground(tcell.ColorDarkGray))

				screen.Show()

				select {
				case ev := <-evCh:
					switch ev := ev.(type) {
					case *tcell.EventResize:
						screen.Sync()
					case *tcell.EventKey:
						if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
							if showEscConfirm(screen, evCh, defStyle) {
								streamer.Close()
								speaker.Clear()
								isPlaying = false
								wantsToPlay = false
								gameState = "WAITING"
								goto lobbyReturn
							}
						} else if ev.Key() == tcell.KeyRune {
							lane, exists := keyMap[ev.Rune()]
							if exists {
								hitIndex := -1
								minDiff := 200
								for i, note := range notes {
									if note.Lane == lane && !noteStates[i].Hit {
										diff := note.TimeMs - currentTimeMs
										if diff < 0 {
											diff = -diff
										}
										if diff < minDiff {
											minDiff = diff
											hitIndex = i
										}
									}
								}
								if hitIndex != -1 {
									noteStates[hitIndex].Hit = true
									combo++
									points := 50
									if minDiff < 50 {
										points = 300
									}
									score += points * (1 + (combo / 10))
								} else {
									combo = 0
								}
							}
						}
					}
				default:
					time.Sleep(16 * time.Millisecond)
				}
			}

			streamer.Close()
			speaker.Clear()
			isPlaying = false
		lobbyReturn:
		}
	}
}

func sendHello(name string) {
	hello := multiplayer.ClientMessage{Type: "HELLO", Name: name}
	b, _ := json.Marshal(hello)
	wsConn.WriteMessage(websocket.TextMessage, b)
}

func buildKeyMap() map[rune]int {
	m := make(map[rune]int)
	for i, k := range gameKeys {
		m[k] = i
		if k >= 'a' && k <= 'z' {
			m[k-32] = i // uppercase
		}
		if k >= 'A' && k <= 'Z' {
			m[k+32] = i // lowercase
		}
	}
	return m
}

func formatTimeMs(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	s := ms / 1000
	m := s / 60
	sec := s % 60
	return fmt.Sprintf("%02d:%02d", m, sec)
}

func showEscConfirm(s tcell.Screen, evCh chan tcell.Event, style tcell.Style) bool {
	for {
		w, h := s.Size()
		boxW := 40
		boxH := 7
		boxX := w/2 - boxW/2
		boxY := h/2 - boxH/2

		// Background
		for y := boxY; y < boxY+boxH; y++ {
			for x := boxX; x < boxX+boxW; x++ {
				s.SetContent(x, y, ' ', nil, style.Background(tcell.ColorBlack))
			}
		}

		// Border
		borderStyle := style.Foreground(tcell.ColorGreen)
		for x := boxX + 1; x < boxX+boxW-1; x++ {
			s.SetContent(x, boxY, '═', nil, borderStyle)
			s.SetContent(x, boxY+boxH-1, '═', nil, borderStyle)
		}
		for y := boxY + 1; y < boxY+boxH-1; y++ {
			s.SetContent(boxX, y, '║', nil, borderStyle)
			s.SetContent(boxX+boxW-1, y, '║', nil, borderStyle)
		}
		s.SetContent(boxX, boxY, '╔', nil, borderStyle)
		s.SetContent(boxX+boxW-1, boxY, '╗', nil, borderStyle)
		s.SetContent(boxX, boxY+boxH-1, '╚', nil, borderStyle)
		s.SetContent(boxX+boxW-1, boxY+boxH-1, '╝', nil, borderStyle)

		title := "[ DESCONECTAR? ]"
		drawText(s, w/2-len(title)/2, boxY+1, title, style.Foreground(tcell.ColorYellow))
		msg := "Voltar ao Lobby?"
		drawText(s, w/2-len(msg)/2, boxY+3, msg, style)
		opts := "[S] Sim    [N] Não"
		drawText(s, w/2-len(opts)/2, boxY+5, opts, style.Foreground(tcell.ColorTeal))

		s.Show()

		ev := <-evCh
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyRune {
				if ev.Rune() == 's' || ev.Rune() == 'S' {
					return true
				}
				if ev.Rune() == 'n' || ev.Rune() == 'N' {
					return false
				}
			}
			if ev.Key() == tcell.KeyEscape {
				return false
			}
		}
	}
}

func drawLobby(s tcell.Screen, style tcell.Style) {
	s.Clear()
	w, h := s.Size()

	title := "[ RÁDIO CENTRAL — BITGUITAR ]"
	drawText(s, w/2-len(title)/2, 2, title, style.Foreground(tcell.ColorTeal))

	var statusMsg string
	if gameState == "WAITING" {
		statusMsg = fmt.Sprintf("SISTEMA EM REPOUSO. Próxima invasão em %ds", countdownMs/1000)
	} else {
		statusMsg = "SISTEMA EM OPERAÇÃO"
	}
	drawText(s, w/2-len(statusMsg)/2, 4, statusMsg, style)

	if nextSong != nil {
		ns := fmt.Sprintf("Próxima Trilha: %s", nextSong.Title)
		drawText(s, w/2-len(ns)/2, 6, ns, style.Foreground(tcell.ColorYellow))
	}

	drawLeaderboard(s, w/2-15, 9, style)

	if wantsToPlay {
		msg := fmt.Sprintf("[ NA FILA COMO: %s ] — Aguarde a próxima trilha...", myName)
		drawText(s, w/2-len(msg)/2, h-5, msg, style.Foreground(tcell.ColorWhite).Bold(true))
	} else {
		msg := "Pressione [J] para entrar na fila  |  [C] Configurar Teclas  |  [ESC] Sair"
		drawText(s, w/2-len(msg)/2, h-5, msg, style.Foreground(tcell.ColorYellow))
	}

	footer := "BitGuitar — Um projeto do laboratório de jogos da crom.run"
	drawText(s, w/2-len(footer)/2, h-2, footer, style.Foreground(tcell.ColorDarkGray))

	s.Show()
}

func drawSpectatorView(s tcell.Screen, style tcell.Style) {
	s.Clear()
	w, h := s.Size()

	title := "[ MODO ESPECTADOR ]"
	drawText(s, w/2-len(title)/2, 2, title, style.Foreground(tcell.ColorTeal))

	if currentSong != nil {
		trackMsg := fmt.Sprintf("Tocando agora: %s", currentSong.Title)
		drawText(s, w/2-len(trackMsg)/2, 4, trackMsg, style)

		currentTimeMs := int64(time.Now().UnixMilli() - startTimeMs)
		elapsed := formatTimeMs(currentTimeMs)
		total := formatTimeMs(durationMs)
		timerMsg := fmt.Sprintf("Tempo: %s / %s", elapsed, total)
		drawText(s, w/2-len(timerMsg)/2, 6, timerMsg, style.Foreground(tcell.ColorTeal))
	}

	if nextSong != nil {
		ns := fmt.Sprintf("Próxima Trilha: %s", nextSong.Title)
		drawText(s, w/2-len(ns)/2, 8, ns, style.Foreground(tcell.ColorYellow))
	}

	drawLeaderboard(s, w/2-15, 11, style)

	msg := "Pressione [J] para entrar na fila da próxima  |  [ESC] Sair"
	drawText(s, w/2-len(msg)/2, h-3, msg, style.Foreground(tcell.ColorYellow))

	footer := "BitGuitar — Um projeto do laboratório de jogos da crom.run"
	drawText(s, w/2-len(footer)/2, h-1, footer, style.Foreground(tcell.ColorDarkGray))

	s.Show()
}

func showConfigScreen(s tcell.Screen, evCh chan tcell.Event, style tcell.Style) {
	for {
		s.Clear()
		w, h := s.Size()

		drawText(s, w/2-15, 3, "=== CONFIGURAÇÃO DE TECLAS ===", style.Foreground(tcell.ColorTeal))
		drawText(s, w/2-15, 5, "Teclas atuais:", style)

		for i, k := range gameKeys {
			msg := fmt.Sprintf("  Coluna %d: [%c]", i+1, k)
			drawText(s, w/2-15, 7+i, msg, style)
		}

		drawText(s, w/2-15, 13, "Pressione [1]-[4] para alterar uma coluna", style.Foreground(tcell.ColorYellow))
		drawText(s, w/2-15, 14, "Pressione [S] para salvar e voltar", style.Foreground(tcell.ColorYellow))
		drawText(s, w/2-15, 15, "Pressione [ESC] para cancelar", style.Foreground(tcell.ColorYellow))

		footer := "BitGuitar — crom.run"
		drawText(s, w/2-len(footer)/2, h-1, footer, style.Foreground(tcell.ColorDarkGray))

		s.Show()

		ev := <-evCh
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				return
			}
			if ev.Key() == tcell.KeyRune {
				r := ev.Rune()
				if r >= '1' && r <= '4' {
					col := int(r - '1')
					s.Clear()
					msg := fmt.Sprintf("Pressione a nova tecla para a Coluna %d:", col+1)
					drawText(s, w/2-len(msg)/2, h/2, msg, style)
					s.Show()

					keyEv := <-evCh
					if ke, ok := keyEv.(*tcell.EventKey); ok && ke.Key() == tcell.KeyRune {
						gameKeys[col] = ke.Rune()
					}
				}
				if r == 's' || r == 'S' {
					saveKeyConfig()
					return
				}
			}
		}
	}
}

func checkExit(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
			return true
		}
	}
	return false
}

func drawText(s tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, c := range text {
		s.SetContent(x+i, y, c, nil, style)
	}
}

func drawLeaderboard(s tcell.Screen, x, y int, style tcell.Style) {
	drawText(s, x, y, "--- TOP HACKERS ---", style.Foreground(tcell.ColorYellow))
	boardMu.Lock()
	defer boardMu.Unlock()
	rank := 0
	for _, p := range board {
		if (p.Name == "Spectator" || p.Name == "Anon") && p.Score == 0 {
			continue
		}
		rank++
		if rank > 10 {
			break
		}
		color := style.Foreground(tcell.ColorGreen)
		if p.Name == myName {
			color = style.Foreground(tcell.ColorWhite).Bold(true)
		}
		row := fmt.Sprintf("%d. %-15s %06d [%dx]", rank, p.Name, p.Score, p.Combo)
		drawText(s, x, y+rank, row, color)
	}
}
