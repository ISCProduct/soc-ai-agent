package shadowscorer

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestFeaturesMatchGolden(t *testing.T) {
	raw, err := os.ReadFile("../../../../../rag/testdata/scorer_feature_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		FNV []struct {
			Input string `json:"input"`
			Hash  uint32 `json:"hash"`
		} `json:"fnv1a32"`
		Normalize []struct {
			Input  string `json:"input"`
			Output string `json:"output"`
		} `json:"normalize"`
		Index []struct {
			Namespace string `json:"namespace"`
			Gram      string `json:"gram"`
			Index     int    `json:"index"`
		} `json:"feature_index"`
		Cases []struct {
			Name  string `json:"name"`
			Turns []struct {
				Q string `json:"q"`
				A string `json:"a"`
			} `json:"turns"`
			Features [][3]float64 `json:"features"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for _, c := range golden.FNV {
		if got := fnv1a32([]byte(c.Input)); got != c.Hash {
			t.Errorf("fnv %q = %d, want %d", c.Input, got, c.Hash)
		}
	}
	for _, c := range golden.Normalize {
		if got := normalize(c.Input); got != c.Output {
			t.Errorf("normalize %q = %q, want %q", c.Input, got, c.Output)
		}
	}
	for _, c := range golden.Index {
		if got := featureIndex(c.Namespace, c.Gram); got != c.Index {
			t.Errorf("index %s:%s = %d, want %d", c.Namespace, c.Gram, got, c.Index)
		}
	}
	for _, c := range golden.Cases {
		turns := make([]Turn, len(c.Turns))
		for i, turn := range c.Turns {
			turns[i] = Turn{Q: turn.Q, A: turn.A}
		}
		got := extract(turns)
		if len(got.indices) != len(c.Features) {
			t.Errorf("%s: features %d, want %d", c.Name, len(got.indices), len(c.Features))
			continue
		}
		for i, row := range c.Features {
			if got.indices[i] != int(row[0]) || got.counts[i] != int(row[1]) {
				t.Errorf("%s: [%d] index/count = %d/%d, want %v/%v", c.Name, i, got.indices[i], got.counts[i], row[0], row[1])
			}
			if math.Abs(got.values[i]-row[2]) > 1e-12 {
				t.Errorf("%s: [%d] value = %v, want %v", c.Name, i, got.values[i], row[2])
			}
		}
	}
}
