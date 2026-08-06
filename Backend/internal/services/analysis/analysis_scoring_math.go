package analysis

import (
	"Backend/domain/entity"
	"encoding/json"
	"math"
	"strings"
)

func parseEmbedding(raw string) ([]float64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var vec []float64
	if err := json.Unmarshal([]byte(raw), &vec); err != nil {
		return nil, err
	}
	return vec, nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return clamp01(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func averageCategoryScore(scoreMap map[string]float64, categories []string) float64 {
	if len(categories) == 0 {
		return 0
	}
	var sum float64
	var count float64
	for _, category := range categories {
		score, ok := scoreMap[category]
		if !ok {
			continue
		}
		sum += clamp01(score / 100.0)
		count++
	}
	if count == 0 {
		return 0
	}
	return clamp01(sum / count)
}

// Exported wrappers for testing from external packages.

func BuildScoreComment(scores AnalysisScores) string { return buildScoreComment(scores) }
func BuildJobSuitabilityComment(scores []entity.UserWeightScore) (string, []JobSuitabilityRole) {
	return buildJobSuitabilityComment(scores)
}
func ParseEmbedding(raw string) ([]float64, error) { return parseEmbedding(raw) }
func CosineSimilarity(a, b []float64) float64      { return cosineSimilarity(a, b) }
func Clamp01(value float64) float64                { return clamp01(value) }
func AverageCategoryScore(scoreMap map[string]float64, categories []string) float64 {
	return averageCategoryScore(scoreMap, categories)
}
