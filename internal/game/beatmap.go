package game

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

type Note struct {
	TimeMs int `json:"time"`
	Lane   int `json:"lane"`
	Type   int `json:"type"` // 0 ou 1
}

type Song struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Beatmap  []Note `json:"beatmap"`
}

// Pseudo-random number generator (Mulberry32 portado para Go)
type Mulberry32 struct {
	state uint32
}

func (m *Mulberry32) Next() float64 {
	m.state += 0x6D2B79F5
	t := m.state
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	result := ((t ^ (t >> 14)) >> 0)
	return float64(result) / 4294967296.0
}

func GenerateBeatmap(filename string, filepath string) ([]Note, string, error) {
	// Obter hash MD5 do arquivo
	f, err := os.Open(filepath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, "", err
	}
	hashString := hex.EncodeToString(h.Sum(nil))

	// Criar seed a partir do hash
	var seed uint32 = 0
	for i := 0; i < len(hashString); i++ {
		seed = (seed << 5) - seed + uint32(hashString[i])
	}

	rng := Mulberry32{state: seed}
	var notes []Note

	bpm := 120.0
	beatIntervalMs := 60000.0 / bpm
	maxDurationMs := 5.0 * 60.0 * 1000.0 // Gera 5 minutos de notas

	currentTime := 1000.0 // Começa 1s depois

	for currentTime < maxDurationMs {
		if rng.Next() > 0.3 {
			lane := int(rng.Next() * 4) // 0 a 3
			noteType := 0
			if rng.Next() > 0.5 {
				noteType = 1
			}
			notes = append(notes, Note{
				TimeMs: int(currentTime),
				Lane:   lane,
				Type:   noteType,
			})
		}

		if rng.Next() > 0.7 {
			currentTime += beatIntervalMs / 2.0
		} else {
			currentTime += beatIntervalMs
		}
	}

	title := filename
	if len(title) > 4 {
		title = title[:len(title)-4] // remove extensão .mp3
	}

	return notes, title, nil
}
