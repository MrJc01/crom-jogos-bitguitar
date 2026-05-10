package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/crom/bitguitar/internal/multiplayer"
	"github.com/gorilla/websocket"
)

func main() {
	botCount := 5
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil {
			botCount = n
		}
	}

	if botCount <= 0 {
		fmt.Println("Bot Swarm desligado.")
		return
	}

	botNames := []string{"Neo", "Trinity", "Morpheus", "Cypher", "Smith", "Oracle", "Dozer", "Tank", "Switch", "Apoc"}

	fmt.Printf("Iniciando Bot Swarm (%d bots)...\n", botCount)

	for i := 0; i < botCount; i++ {
		baseName := botNames[i%len(botNames)]
		if i >= len(botNames) {
			baseName = fmt.Sprintf("%s-%d", baseName, i)
		}
		name := fmt.Sprintf("%s (BOT)", baseName)
		go startBot(name)
		time.Sleep(300 * time.Millisecond) // stagger connects
	}

	// Trava o main
	select {}
}

func startBot(name string) {
	u := "ws://localhost:8080/ws"
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		log.Printf("[%s] Erro ao conectar: %v", name, err)
		return
	}
	defer c.Close()

	// Enviar identificação
	hello := multiplayer.ClientMessage{Type: "HELLO", Name: name}
	b, _ := json.Marshal(hello)
	c.WriteMessage(websocket.TextMessage, b)

	score := 0
	combo := 0
	isPlaying := false

	// Rotina de receber estado
	go func() {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var sm multiplayer.StateMessage
			json.Unmarshal(msg, &sm)

			if sm.Type == "STATE" {
				if sm.State == "PLAYING" {
					if !isPlaying {
						isPlaying = true
						score = 0
						combo = 0
					}
				} else {
					isPlaying = false
				}
			}
		}
	}()

	// Rotina de jogar
	for {
		time.Sleep(500 * time.Millisecond)
		if isPlaying {
			// Simula jogar
			hitChance := rand.Float32()
			if hitChance > 0.1 { // 90% chance de acertar uma nota imaginária
				combo++
				score += 100 * (1 + (combo / 10))
			} else {
				combo = 0 // errou
			}

			update := multiplayer.ClientMessage{Type: "SCORE", Score: score, Combo: combo}
			out, _ := json.Marshal(update)
			c.WriteMessage(websocket.TextMessage, out)
		}
	}
}
