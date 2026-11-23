package shadowscorer

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestLoadRejectsRelativePath(t *testing.T) {
	if _, err := Load("scorer.bin", rubricKeys(), 0, 5); err == nil {
		t.Fatal("相対パスを受け入れた")
	}
}

func TestLoadRejectsBadChecksum(t *testing.T) {
	path := writeModel(t, map[int]float64{0: 1}, []float64{-1, -1, -1, -1, -1})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff
	if _, err := decode(data, rubricKeys(), 0, 5); err == nil {
		t.Fatal("壊した重みを受け入れた")
	}
}

func TestScoreCountsThresholds(t *testing.T) {
	path := writeModel(t, map[int]float64{featureIndex("a", "a"): 1}, []float64{-1, -1, 10, 10, 10})
	m, err := Load(path, rubricKeys(), 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	// 添字 0 の特徴だけが 1。重み 1 なので z = 1。閾値を上回るのは2つ。
	out, err := NewFileScorer(m).Score(context.Background(), ScoreInput{Turns: []Turn{{A: "a"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out.ModelVersion == "" {
		t.Fatal("モデルの版が空")
	}
	got := out.Scores["logic"]
	if got.Score != 2 {
		t.Fatalf("logic score = %d, want 2", got.Score)
	}
	if got.Confidence <= 0 {
		t.Fatalf("confidence = %v", got.Confidence)
	}
}

func TestModelVersionIncludesThresholds(t *testing.T) {
	weights := map[int]float64{0: 1}
	low := writeModel(t, weights, []float64{-1, -1, -1, -1, -1})
	high := writeModel(t, weights, []float64{1, 1, 1, 1, 1})
	a, err := Load(low, rubricKeys(), 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(high, rubricKeys(), 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if a.version == b.version {
		t.Fatal("閾値だけが違うモデルを同じ版にした")
	}
}

func TestScoreTrainedModelFile(t *testing.T) {
	path := os.Getenv("SCORER_MODEL_PATH")
	if path == "" {
		t.Skip("SCORER_MODEL_PATH が無いので、学習済みファイルの配線確認は飛ばす")
	}
	m, err := Load(path, rubricKeys(), 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewFileScorer(m).Score(context.Background(), ScoreInput{Turns: []Turn{{
		Q: "学生時代に力を入れたことは？",
		A: "架空のサークルで会計を担当しました。",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if out.ModelVersion == "" || len(out.Scores) != len(rubricKeys()) {
		t.Fatalf("out = %+v", out)
	}
	for _, key := range rubricKeys() {
		score := out.Scores[key]
		if score.Score < 0 || score.Score > 5 {
			t.Fatalf("%s score = %d", key, score.Score)
		}
	}
	result := os.Getenv("SCORER_RESULT_PATH")
	if result == "" {
		return
	}
	body, err := json.Marshal(map[string]any{
		"model_version": out.ModelVersion,
		"scores":        out.Scores,
		"note":          "人工データで学習したモデルの配線確認。精度の主張ではない。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(result, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func rubricKeys() []string {
	return []string{"logic", "specificity", "ownership", "communication", "enthusiasm"}
}

func writeModel(t *testing.T, weights map[int]float64, thresholds []float64) string {
	t.Helper()
	keys := rubricKeys()
	blob := make([]byte, len(keys)*hashDim*2)
	for idx, v := range weights {
		bits := float64To16(v)
		for a := range keys {
			binary.LittleEndian.PutUint16(blob[(a*hashDim+idx)*2:], bits)
		}
	}
	sum := sha256.Sum256(blob)
	header := map[string]any{
		"format":         formatName,
		"format_version": formatVersion,
		"rubric": map[string]any{
			"version":   rubricFingerprint(keys, 0, 5),
			"keys":      keys,
			"score_min": 0,
			"score_max": 5,
		},
		"features": map[string]any{
			"spec_version":    featureSpecVersion,
			"hash":            "fnv1a32",
			"dim":             hashDim,
			"ngram_min":       ngramMin,
			"ngram_max":       ngramMax,
			"unit":            "codepoint",
			"normalization":   []string{"surrogate_to_fffd", "nfkc", "ascii_lower", "collapse_whitespace", "trim"},
			"namespaces":      map[string]string{"question": "q", "answer": "a"},
			"value":           "1+ln(count), l2_normalized",
			"unicode_version": "15.0.0",
		},
		"thresholds": map[string][]float64{},
		"weights": map[string]any{
			"dtype":       "float16",
			"byte_order":  "little",
			"layout":      "axis_major",
			"shape":       []int{len(keys), hashDim},
			"byte_length": len(blob),
			"sha256":      hex.EncodeToString(sum[:]),
		},
		"label_source": "llm",
	}
	// テスト機の Unicode 版に合わせる。本番の読み込みはファイル側の版と突き合わせる。
	header["features"].(map[string]any)["unicode_version"] = norm.Version
	for _, key := range keys {
		header["thresholds"].(map[string][]float64)[key] = thresholds
	}
	head, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	file := append([]byte(magic), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(file[len(magic):], uint32(len(head)))
	file = append(file, head...)
	file = append(file, blob...)
	path := filepath.Join(t.TempDir(), "scorer.bin")
	if err := os.WriteFile(path, file, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func float64To16(f float64) uint16 {
	if f == 0 {
		return 0
	}
	sign := uint16(0)
	if f < 0 {
		sign = 0x8000
		f = -f
	}
	exp := math.Ilogb(f)
	frac := f/math.Ldexp(1, exp) - 1
	bits := int(math.Round(frac * 1024))
	if bits == 1024 {
		exp++
		bits = 0
	}
	return sign | uint16((exp+15)<<10) | uint16(bits)
}
