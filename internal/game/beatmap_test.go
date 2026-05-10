package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMulberry32(t *testing.T) {
	// O gerador Mulberry32 é determinístico por design.
	// Uma seed específica sempre produz a mesma sequência de outputs.
	rng1 := Mulberry32{state: 12345}
	rng2 := Mulberry32{state: 12345}

	v1_1 := rng1.Next()
	v1_2 := rng1.Next()

	v2_1 := rng2.Next()
	v2_2 := rng2.Next()

	if v1_1 != v2_1 {
		t.Errorf("Determinismo quebrado: Esperado %v, obtido %v", v1_1, v2_1)
	}
	if v1_2 != v2_2 {
		t.Errorf("Determinismo quebrado na segunda iteracao: Esperado %v, obtido %v", v1_2, v2_2)
	}

	if v1_1 == v1_2 {
		t.Errorf("Gerador estático. Outputs iguais gerados em sequência: %v", v1_1)
	}
}

func TestGenerateBeatmap(t *testing.T) {
	// Cria arquivo temporário pra mockar a música
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mock_song.mp3")

	// Preenche com alguns dados aleatórios
	mockData := []byte("fake_audio_data_12345")
	if err := os.WriteFile(filePath, mockData, 0644); err != nil {
		t.Fatalf("Erro ao criar mock: %v", err)
	}

	notes1, title1, err := GenerateBeatmap("mock_song.mp3", filePath)
	if err != nil {
		t.Fatalf("Erro inesperado em GenerateBeatmap: %v", err)
	}

	if title1 != "mock_song" {
		t.Errorf("Esperava titulo 'mock_song', obtido '%s'", title1)
	}

	if len(notes1) == 0 {
		t.Error("Nenhuma nota gerada.")
	}

	// Testa a propriedade temporal (todas notas em ordem crescente/igual)
	lastTime := -1
	for _, note := range notes1 {
		if note.TimeMs < lastTime {
			t.Errorf("Falha temporal: nota com tempo %d veio depois de %d", note.TimeMs, lastTime)
		}
		lastTime = note.TimeMs
		
		if note.Lane < 0 || note.Lane > 3 {
			t.Errorf("Lane invalido: %d", note.Lane)
		}
		if note.Type != 0 && note.Type != 1 {
			t.Errorf("Tipo invalido: %d", note.Type)
		}
	}

	// Testa o Determinismo: Chamar de novo deve dar o EXATO mesmo mapa
	notes2, _, _ := GenerateBeatmap("mock_song.mp3", filePath)
	
	if len(notes1) != len(notes2) {
		t.Fatalf("Determinismo falhou. Arrays de tamanho diferente: %d vs %d", len(notes1), len(notes2))
	}
	for i := range notes1 {
		if notes1[i] != notes2[i] {
			t.Errorf("Determinismo falhou na nota %d: %v vs %v", i, notes1[i], notes2[i])
		}
	}
}

func TestGenerateBeatmap_FileNotFound(t *testing.T) {
	_, _, err := GenerateBeatmap("missing.mp3", "/caminho/invalido/missing.mp3")
	if err == nil {
		t.Error("Esperava erro por arquivo não encontrado, obtido nil")
	}
}
