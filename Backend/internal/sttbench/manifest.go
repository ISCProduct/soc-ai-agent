package sttbench

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Case は manifest.jsonl の1行。
type Case struct {
	ID            string   `json:"id"`
	File          string   `json:"file"`
	WebmFile      string   `json:"webm_file"`
	ReferenceText string   `json:"reference_text"`
	Tags          []string `json:"tags"`
	Checks        []string `json:"checks"`
}

// LoadManifest は manifest.jsonl を読む。
// 1行でも壊れていれば止める。欠けたケースを黙って飛ばすと、
// 「評価した件数」が実態とずれる。
func LoadManifest(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []Case
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(text), &c); err != nil {
			return nil, fmt.Errorf("manifest %d行目: %w", line, err)
		}
		if c.ID == "" || c.ReferenceText == "" {
			return nil, fmt.Errorf("manifest %d行目: id と reference_text は必須", line)
		}
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("manifest にケースがありません: %s", path)
	}
	return cases, nil
}

// Audio は評価に使う音声1件の実体情報。
type Audio struct {
	ID          string  `json:"id"`
	Path        string  `json:"-"`
	Format      string  `json:"format"`
	Bytes       int     `json:"bytes"`
	DurationSec float64 `json:"duration_sec"`
	Data        []byte  `json:"-"`
}

// InspectAll は各ケースの音声を読み、形式と長さを確認する。
//
// 長さを見るのは、無音や途中切れをモデルの精度差と誤認しないため。
// WAVはヘッダから正確に出せる。WEBMは可変長コンテナで簡単に読めないため
// 長さ0として扱い、長さに依存する判定（認識失敗の下限）を無効化する。
func InspectAll(cases []Case, baseDir, format string) ([]Audio, error) {
	out := make([]Audio, 0, len(cases))
	for _, c := range cases {
		rel := c.File
		if format == "webm" {
			rel = c.WebmFile
		}
		if rel == "" {
			return nil, fmt.Errorf("%s: %s 形式のファイルが manifest にありません", c.ID, format)
		}
		p := filepath.Join(baseDir, rel)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.ID, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("%s: 音声が空です (%s)", c.ID, p)
		}
		a := Audio{ID: c.ID, Path: p, Format: format, Bytes: len(data), Data: data}
		if format == "wav" {
			sec, err := WavDurationSec(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", c.ID, err)
			}
			a.DurationSec = sec
		}
		out = append(out, a)
	}
	return out, nil
}

// WavDurationSec は WAV ヘッダから再生時間を求める。
//
// フィクスチャが無音・途中切れになっていないかを確認するために使う。
// 対応は PCM の RIFF/WAVE のみ。
func WavDurationSec(b []byte) (float64, error) {
	if len(b) < 44 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return 0, fmt.Errorf("WAVヘッダが不正")
	}
	var byteRate uint32
	var dataSize uint32
	pos := 12
	for pos+8 <= len(b) {
		id := string(b[pos : pos+4])
		size := binary.LittleEndian.Uint32(b[pos+4 : pos+8])
		body := pos + 8
		switch id {
		case "fmt ":
			if body+16 > len(b) {
				return 0, fmt.Errorf("fmtチャンクが壊れている")
			}
			byteRate = binary.LittleEndian.Uint32(b[body+8 : body+12])
		case "data":
			dataSize = size
		}
		pos = body + int(size)
		if size%2 == 1 {
			pos++ // RIFF は奇数サイズのチャンクを1バイト詰める
		}
	}
	if byteRate == 0 || dataSize == 0 {
		return 0, fmt.Errorf("再生時間を判定できない（byteRate=%d dataSize=%d）", byteRate, dataSize)
	}
	return float64(dataSize) / float64(byteRate), nil
}
