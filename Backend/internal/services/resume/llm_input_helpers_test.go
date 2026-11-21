package resume

import (
	"Backend/internal/models"
	"Backend/internal/services/piimask"
	"testing"
)

// maskedForTest は、テストの本文を本番と同じ規則で伏字にして llmBlocks にする。
func maskedForTest(t testing.TB, blocks []models.ResumeTextBlock) llmBlocks {
	t.Helper()
	m, err := piimask.ForLLM(piimask.Subject{})
	if err != nil {
		t.Fatalf("ForLLM: %v", err)
	}
	return maskBlocksWith(m, blocks)
}
