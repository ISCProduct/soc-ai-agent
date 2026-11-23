package shadowscorer

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

const (
	magic          = "SOCSCORE"
	formatName     = "soc-interview-scorer"
	formatVersion  = 1
	maxHeaderBytes = 1 << 20
	maxModelBytes  = 2_700_000
	labelSourceLLM = "llm"
)

// AxisScore は1軸のスコアと、確信度の目安（期待値）。
type AxisScore struct {
	Score      int     `json:"score"`
	Confidence float64 `json:"confidence"`
}

// ScoreInput は面接1回ぶんの発話。
type ScoreInput struct {
	Turns []Turn
}

// ScoreOutput は軸ごとのスコアと、重みの SHA-256（モデルの版）。
type ScoreOutput struct {
	Scores       map[string]AxisScore
	ModelVersion string
}

// InterviewScorer はローカル採点の口。LLM の採点とは別物。
type InterviewScorer interface {
	Score(ctx context.Context, in ScoreInput) (ScoreOutput, error)
}

// Model は学習済みファイルを読んだ採点器。
type Model struct {
	keys       []string
	scoreMin   int
	thresholds [][]float64
	weights    []uint16 // axis-major float16 bits
	version    string   // 重みの SHA-256
}

type fileScorer struct {
	model *Model
}

// NewFileScorer は読み込み済みモデルを採点口にする。
func NewFileScorer(m *Model) InterviewScorer {
	return fileScorer{model: m}
}

func (f fileScorer) Score(ctx context.Context, in ScoreInput) (ScoreOutput, error) {
	if err := ctx.Err(); err != nil {
		return ScoreOutput{}, err
	}
	if f.model == nil {
		return ScoreOutput{}, errors.New("採点モデルが読み込まれていません")
	}
	return f.model.Score(in.Turns), nil
}

// LoadFromEnv はフラグがオフなら (nil, nil)。オンで読めなければエラー。
func LoadFromEnv(rubricKeys []string, scoreMin, scoreMax int) (InterviewScorer, error) {
	raw := strings.TrimSpace(os.Getenv("INTERVIEW_SHADOW_SCORER"))
	if raw != "1" && !strings.EqualFold(raw, "true") {
		return nil, nil
	}
	path := strings.TrimSpace(os.Getenv("INTERVIEW_SHADOW_MODEL_PATH"))
	m, err := Load(path, rubricKeys, scoreMin, scoreMax)
	if err != nil {
		return nil, err
	}
	return NewFileScorer(m), nil
}

// Load はモデルファイルを読む。形・Unicode の版・ルーブリック・重みの SHA-256 が
// この実装と合わなければエラーにし、そのモデルでは採点しない。
func Load(path string, rubricKeys []string, scoreMin, scoreMax int) (*Model, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return nil, errors.New("採点モデルのパスは絶対パスにしてください")
	}
	// 起動設定で渡されたモデルファイルだけを読む。相対パスは上で拒否している。
	data, err := os.ReadFile(cleaned) // #nosec G304,G703 -- 絶対パスのモデルファイル
	if err != nil {
		return nil, fmt.Errorf("採点モデルを開けません: %w", err)
	}
	if len(data) > maxModelBytes {
		return nil, fmt.Errorf("採点モデルが %d バイトで、上限 %d バイトを超えています", len(data), maxModelBytes)
	}
	return decode(data, rubricKeys, scoreMin, scoreMax)
}

func decode(data []byte, rubricKeys []string, scoreMin, scoreMax int) (*Model, error) {
	if len(data) < len(magic)+4 || string(data[:len(magic)]) != magic {
		return nil, errors.New("採点モデルのファイルではありません")
	}
	headLen := binary.LittleEndian.Uint32(data[len(magic):])
	start := len(magic) + 4
	if headLen > maxHeaderBytes || start+int(headLen) > len(data) {
		return nil, errors.New("ヘッダの長さが不正です")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data[start:start+int(headLen)], &root); err != nil {
		return nil, fmt.Errorf("ヘッダを JSON として読めません: %w", err)
	}
	var format string
	var version int
	if err := json.Unmarshal(root["format"], &format); err != nil || format != formatName {
		return nil, errors.New("ヘッダの format が違います")
	}
	if err := json.Unmarshal(root["format_version"], &version); err != nil || version != formatVersion {
		return nil, fmt.Errorf("モデルの形の版 %d には対応していません", version)
	}
	var rubric struct {
		Version  string   `json:"version"`
		Keys     []string `json:"keys"`
		ScoreMin int      `json:"score_min"`
		ScoreMax int      `json:"score_max"`
	}
	if err := json.Unmarshal(root["rubric"], &rubric); err != nil || len(rubric.Keys) == 0 {
		return nil, errors.New("ルーブリックの軸がありません")
	}
	if rubric.ScoreMin != scoreMin || rubric.ScoreMax != scoreMax {
		return nil, errors.New("ルーブリックの値域がこの実装と一致しません")
	}
	if strings.Join(rubric.Keys, ",") != strings.Join(rubricKeys, ",") {
		return nil, errors.New("ルーブリックの軸の並びがこの実装と一致しません")
	}
	if rubric.Version != rubricFingerprint(rubricKeys, scoreMin, scoreMax) {
		return nil, errors.New("ルーブリックの版がこの実装と一致しません")
	}
	var feats map[string]any
	if err := json.Unmarshal(root["features"], &feats); err != nil {
		return nil, errors.New("特徴量の設定が、この実装と一致しません")
	}
	unicodeVersion, _ := feats["unicode_version"].(string)
	delete(feats, "unicode_version")
	if !featureSpecMatches(feats) {
		return nil, errors.New("特徴量の設定が、この実装と一致しません")
	}
	if unicodeVersion != norm.Version {
		return nil, fmt.Errorf("モデルの Unicode の版 %q が、この実行環境の版 %q と一致しません", unicodeVersion, norm.Version)
	}
	var thresholds map[string][]float64
	if err := json.Unmarshal(root["thresholds"], &thresholds); err != nil {
		return nil, errors.New("閾値の値が不正です")
	}
	nThresh := scoreMax - scoreMin
	rows := make([][]float64, len(rubric.Keys))
	for i, key := range rubric.Keys {
		row := thresholds[key]
		if len(row) != nThresh {
			return nil, errors.New("閾値の数が不正です")
		}
		for _, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, errors.New("閾値の値が不正です")
			}
		}
		rows[i] = row
	}
	var wmeta struct {
		Dtype      string `json:"dtype"`
		ByteOrder  string `json:"byte_order"`
		Layout     string `json:"layout"`
		ByteLength int    `json:"byte_length"`
		SHA256     string `json:"sha256"`
	}
	if err := json.Unmarshal(root["weights"], &wmeta); err != nil {
		return nil, errors.New("重みの形式に対応していません")
	}
	if wmeta.Dtype != "float16" || wmeta.ByteOrder != "little" || wmeta.Layout != "axis_major" {
		return nil, errors.New("重みの形式に対応していません")
	}
	blob := data[start+int(headLen):]
	wantLen := len(rubric.Keys) * hashDim * 2
	if wmeta.ByteLength != wantLen || len(blob) != wantLen {
		return nil, fmt.Errorf("重みの長さが合いません（ヘッダ %d、実際 %d）", wmeta.ByteLength, len(blob))
	}
	sum := sha256.Sum256(blob)
	if hex.EncodeToString(sum[:]) != wmeta.SHA256 {
		return nil, errors.New("重みのチェックサム（SHA-256）が合いません")
	}
	fileSum := sha256.Sum256(data)
	weights := make([]uint16, len(blob)/2)
	for i := range weights {
		weights[i] = binary.LittleEndian.Uint16(blob[i*2:])
		v := float16To64(weights[i])
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("重みに有限でない値があります")
		}
	}
	var label string
	_ = json.Unmarshal(root["label_source"], &label)
	if label != labelSourceLLM {
		return nil, errors.New("学習ラベルの出所が llm ではありません")
	}
	return &Model{
		keys:       append([]string(nil), rubric.Keys...),
		scoreMin:   scoreMin,
		thresholds: rows,
		weights:    weights,
		version:    hex.EncodeToString(fileSum[:]),
	}, nil
}

// Score は軸ごとのスコアを返す。失敗では空の Scores を返さない（呼び出し側がエラーにする）。
func (m *Model) Score(turns []Turn) ScoreOutput {
	feats := extract(turns)
	out := ScoreOutput{
		Scores:       make(map[string]AxisScore, len(m.keys)),
		ModelVersion: m.version,
	}
	for a, key := range m.keys {
		z := dot(m.weights[a*hashDim:(a+1)*hashDim], feats)
		above := 0
		var expected float64
		for _, theta := range m.thresholds[a] {
			if z > theta {
				above++
			}
			expected += sigmoid(z - theta)
		}
		out.Scores[key] = AxisScore{Score: m.scoreMin + above, Confidence: expected}
	}
	return out
}

func dot(weights []uint16, feats sparseFeatures) float64 {
	if len(feats.indices) == 0 {
		return 0
	}
	var z float64
	for i, idx := range feats.indices {
		z += float16To64(weights[idx]) * feats.values[i]
	}
	return z
}

func sigmoid(x float64) float64 {
	if x > 35 {
		x = 35
	} else if x < -35 {
		x = -35
	}
	return 1 / (1 + math.Exp(-x))
}

func float16To64(b uint16) float64 {
	sign := 1.0
	if b&0x8000 != 0 {
		sign = -1
	}
	exp := int((b >> 10) & 0x1f)
	frac := b & 0x3ff
	switch exp {
	case 0:
		if frac == 0 {
			return math.Copysign(0, sign)
		}
		return sign * math.Ldexp(float64(frac)/1024, -14)
	case 31:
		if frac == 0 {
			return math.Inf(int(sign))
		}
		return math.NaN()
	default:
		return sign * math.Ldexp(1+float64(frac)/1024, exp-15)
	}
}

func rubricFingerprint(keys []string, scoreMin, scoreMax int) string {
	canonical := strings.Join(keys, ",") + fmt.Sprintf("|%d-%d", scoreMin, scoreMax)
	sum := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}

func featureSpecMatches(feats map[string]any) bool {
	if len(feats) != 9 {
		return false
	}
	if asInt(feats["spec_version"]) != featureSpecVersion || feats["hash"] != "fnv1a32" || asInt(feats["dim"]) != hashDim {
		return false
	}
	if asInt(feats["ngram_min"]) != ngramMin || asInt(feats["ngram_max"]) != ngramMax || feats["unit"] != "codepoint" {
		return false
	}
	if feats["value"] != "1+ln(count), l2_normalized" {
		return false
	}
	norms, ok := feats["normalization"].([]any)
	wantNorm := []string{"surrogate_to_fffd", "nfkc", "ascii_lower", "collapse_whitespace", "trim"}
	if !ok || len(norms) != len(wantNorm) {
		return false
	}
	for i, w := range wantNorm {
		if norms[i] != w {
			return false
		}
	}
	ns, ok := feats["namespaces"].(map[string]any)
	return ok && len(ns) == 2 && ns["question"] == namespaceQuestion && ns["answer"] == namespaceAnswer
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return -1
	}
}
