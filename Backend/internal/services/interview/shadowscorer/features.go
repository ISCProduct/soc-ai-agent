package shadowscorer

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// 特徴量の手順は rag/training/scorer/features.py と同じにする。
// 手順を変えたら FEATURE_SPEC_VERSION を上げ、ゴールデンを作り直す。
const (
	featureSpecVersion = 1
	hashDimBits        = 16
	hashDim            = 1 << hashDimBits
	ngramMin           = 1
	ngramMax           = 3
	namespaceQuestion  = "q"
	namespaceAnswer    = "a"
)

// Turn は質問と回答の1組。
type Turn struct {
	Q string
	A string
}

type sparseFeatures struct {
	indices []int
	counts  []int
	values  []float64
}

func fnv1a32(data []byte) uint32 {
	h := fnv.New32a()
	_, _ = h.Write(data)
	return h.Sum32()
}

func normalize(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r >= 0xD800 && r <= 0xDFFF {
			b.WriteRune('\uFFFD')
			continue
		}
		b.WriteRune(r)
	}
	text = norm.NFKC.String(b.String())
	b.Reset()
	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		b.WriteRune(r)
	}
	text = b.String()
	var out strings.Builder
	pending := false
	started := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			pending = true
			continue
		}
		if pending && started {
			out.WriteByte(' ')
		}
		pending = false
		started = true
		out.WriteRune(r)
	}
	return out.String()
}

func charNgrams(text string) []string {
	runes := []rune(text)
	var grams []string
	for n := ngramMin; n <= ngramMax; n++ {
		for i := 0; i+n <= len(runes); i++ {
			grams = append(grams, string(runes[i:i+n]))
		}
	}
	return grams
}

func featureIndex(namespace, gram string) int {
	return int(fnv1a32([]byte(namespace+":"+gram)) & (hashDim - 1))
}

func extract(turns []Turn) sparseFeatures {
	counts := map[int]int{}
	for _, turn := range turns {
		for _, pair := range []struct {
			ns, text string
		}{
			{namespaceQuestion, turn.Q},
			{namespaceAnswer, turn.A},
		} {
			for _, gram := range charNgrams(normalize(pair.text)) {
				counts[featureIndex(pair.ns, gram)]++
			}
		}
	}
	if len(counts) == 0 {
		return sparseFeatures{}
	}
	indices := make([]int, 0, len(counts))
	for idx := range counts {
		indices = append(indices, idx)
	}
	// 添字の昇順。ノルムの和の順を Python と揃える。
	for i := 1; i < len(indices); i++ {
		for j := i; j > 0 && indices[j] < indices[j-1]; j-- {
			indices[j], indices[j-1] = indices[j-1], indices[j]
		}
	}
	raw := make([]float64, len(indices))
	gotCounts := make([]int, len(indices))
	var sq float64
	for i, idx := range indices {
		c := counts[idx]
		gotCounts[i] = c
		v := 1 + math.Log(float64(c))
		raw[i] = v
		sq += v * v
	}
	normValue := math.Sqrt(sq)
	values := make([]float64, len(raw))
	if normValue > 0 {
		for i, v := range raw {
			values[i] = v / normValue
		}
	}
	return sparseFeatures{indices: indices, counts: gotCounts, values: values}
}
