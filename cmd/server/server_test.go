package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MrJc01/crom-jogos-bitguitar/internal/game"
)

func setupTestMux(t *testing.T) (*http.ServeMux, string) {
	tmpMusicasDir := t.TempDir()
	// PublicDir doesn't matter much for API test, pass empty temp dir
	tmpPublicDir := t.TempDir()
	
	mux := NewRouter(tmpMusicasDir, tmpPublicDir, nil)
	return mux, tmpMusicasDir
}

func TestApiSongs_EmptyDirectory(t *testing.T) {
	mux, _ := setupTestMux(t)

	req, _ := http.NewRequest("GET", "/api/songs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler retornou status incorreto: obtido %v experado %v", status, http.StatusOK)
	}

	var songs []game.Song
	err := json.NewDecoder(rr.Body).Decode(&songs)
	if err != nil {
		t.Fatalf("Erro ao decodificar JSON vazio: %v", err)
	}

	if len(songs) != 0 {
		t.Errorf("Esperava lista de musicas vazia, obteve %d itens", len(songs))
	}
}

func TestApiSongs_IgnoreNonAudioFiles(t *testing.T) {
	mux, musicasDir := setupTestMux(t)

	// Cria um arquivo txt e um jpg
	os.WriteFile(filepath.Join(musicasDir, "readme.txt"), []byte("teste"), 0644)
	os.WriteFile(filepath.Join(musicasDir, "cover.jpg"), []byte("teste"), 0644)
	// Cria um mp3 valido
	os.WriteFile(filepath.Join(musicasDir, "valid.mp3"), []byte("audio"), 0644)

	req, _ := http.NewRequest("GET", "/api/songs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var songs []game.Song
	json.NewDecoder(rr.Body).Decode(&songs)

	if len(songs) != 1 {
		t.Fatalf("Esperava exatamente 1 musica ignorando txt e jpg, obteve %d", len(songs))
	}

	if songs[0].Filename != "valid.mp3" {
		t.Errorf("Musica retornada incorreta: %s", songs[0].Filename)
	}
}

func TestApiSongs_ValidBeatmapGeneration(t *testing.T) {
	mux, musicasDir := setupTestMux(t)

	os.WriteFile(filepath.Join(musicasDir, "track1.mp3"), []byte("mocked_audio_content"), 0644)

	req, _ := http.NewRequest("GET", "/api/songs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var songs []game.Song
	json.NewDecoder(rr.Body).Decode(&songs)

	if len(songs) != 1 {
		t.Fatalf("Falha ao carregar musica: len=%d", len(songs))
	}

	s := songs[0]
	if len(s.Beatmap) == 0 {
		t.Error("Beatmap gerado vazio")
	}
	if s.URL != "/musicas/track1.mp3" {
		t.Errorf("URL gerada incorreta: %s", s.URL)
	}
	if s.Title != "track1" {
		t.Errorf("Title gerado incorreto: %s", s.Title)
	}
}

func TestPlaySh_Endpoint(t *testing.T) {
	mux, _ := setupTestMux(t)

	req, _ := http.NewRequest("GET", "/play.sh", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("play.sh endpoint retornou %v", status)
	}

	if contentType := rr.Header().Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("Content-Type errado: %s", contentType)
	}

	body := rr.Body.String()
	if len(body) < 10 {
		t.Error("Corpo do script play.sh vazio ou muito curto")
	}
}
