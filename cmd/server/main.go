package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"github.com/joho/godotenv"
	"github.com/MrJc01/crom-jogos-bitguitar/internal/game"
	"github.com/MrJc01/crom-jogos-bitguitar/internal/multiplayer"
)

var (
	Port       = ":8080"
	MusicasDir = "./musicas"
	PublicDir  = "./public"
)

func main() {
	// Carregar .env se existir
	godotenv.Load()

	if envPort := os.Getenv("PORT"); envPort != "" {
		if !strings.HasPrefix(envPort, ":") {
			Port = ":" + envPort
		} else {
			Port = envPort
		}
	}
	if envDir := os.Getenv("SONG_DIR"); envDir != "" {
		MusicasDir = envDir
	}

	// Garantir que diretório de músicas exista
	if _, err := os.Stat(MusicasDir); os.IsNotExist(err) {
		os.Mkdir(MusicasDir, 0755)
	}

	songs := LoadAllSongs(MusicasDir)
	hub := multiplayer.NewHub(songs)

	mux := NewRouter(MusicasDir, PublicDir, hub)

	fmt.Printf("[BitGuitar Server] Hacker training environment running on http://localhost%s\n", Port)
	fmt.Printf("[BitGuitar Server] API endpoint: http://localhost%s/api/songs\n", Port)
	log.Fatal(http.ListenAndServe(Port, mux))
}

func LoadAllSongs(musicasPath string) []game.Song {
	entries, err := os.ReadDir(musicasPath)
	var songs []game.Song
	if err != nil {
		return songs
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".mp3" || ext == ".wav" || ext == ".ogg" {
			filePath := filepath.Join(musicasPath, entry.Name())
			notes, title, err := game.GenerateBeatmap(entry.Name(), filePath)
			if err != nil {
				continue
			}

			hash := md5.Sum([]byte(entry.Name()))
			id := hex.EncodeToString(hash[:])

			song := game.Song{
				ID:       id,
				Filename: entry.Name(),
				Title:    title,
				URL:      "/musicas/" + entry.Name(),
				Beatmap:  notes,
			}
			songs = append(songs, song)
		}
	}
	return songs
}

func NewRouter(musicasPath, publicPath string, hub *multiplayer.Hub) *http.ServeMux {
	mux := http.NewServeMux()

	// Mantemos a API antiga para testes legado, se necessário
	mux.HandleFunc("/api/songs", func(w http.ResponseWriter, r *http.Request) {
		handleSongs(w, r, musicasPath)
	})
	
	mux.HandleFunc("/play.sh", handlePlaySh)

	if hub != nil {
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			multiplayer.ServeWs(hub, w, r)
		})
	}

	// Servir arquivos estáticos (ignorar se as pastas não existirem para testes seguros)
	if _, err := os.Stat(publicPath); !os.IsNotExist(err) {
		mux.Handle("/", http.FileServer(http.Dir(publicPath)))
	}
	if _, err := os.Stat(musicasPath); !os.IsNotExist(err) {
		mux.Handle("/musicas/", http.StripPrefix("/musicas/", http.FileServer(http.Dir(musicasPath))))
	}

	return mux
}

func handleSongs(w http.ResponseWriter, r *http.Request, musicasPath string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	entries, err := os.ReadDir(musicasPath)
	if err != nil {
		http.Error(w, `{"error": "Could not read music directory"}`, http.StatusInternalServerError)
		return
	}

	var songs []game.Song
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".mp3" || ext == ".wav" || ext == ".ogg" {
			filePath := filepath.Join(musicasPath, entry.Name())
			
			notes, title, err := game.GenerateBeatmap(entry.Name(), filePath)
			if err != nil {
				log.Printf("Error processing %s: %v", entry.Name(), err)
				continue
			}

			// Gerar ID simplificado pelo nome
			hash := md5.Sum([]byte(entry.Name()))
			id := hex.EncodeToString(hash[:])

			song := game.Song{
				ID:       id,
				Filename: entry.Name(),
				Title:    title,
				URL:      "/musicas/" + entry.Name(),
				Beatmap:  notes,
			}
			songs = append(songs, song)
		}
	}

	// Sempre retorna array vazio em vez de null no json
	if songs == nil {
		songs = []game.Song{}
	}

	json.NewEncoder(w).Encode(songs)
}

func handlePlaySh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	protocol := "http://"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		protocol = "https://"
	}
	host := r.Host

	script := fmt.Sprintf(`#!/bin/bash
echo -e "\033[0;32m"
echo "====================================="
echo "   BITGUITAR - AI HACKER TRAINING    "
echo "====================================="
echo "Iniciando conexão com %s..."
echo -e "\033[0m"

if ! command -v go &> /dev/null
then
    echo "Erro: Go não está instalado. Por favor instale o Golang para executar o treinamento no terminal."
    exit 1
fi

export BITGUITAR_HOST="%s%s"
go run github.com/MrJc01/crom-jogos-bitguitar/cmd/cli@master "$@"
`, host, protocol, host)
	w.Write([]byte(script))
}
