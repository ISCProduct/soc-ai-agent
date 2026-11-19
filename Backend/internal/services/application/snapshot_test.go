package application

import (
	"encoding/json"
	"testing"
	"time"

	"Backend/internal/models"
)

func TestBuildScoreSnapshotDocument_版とマッチ値を残し設問本文は入れない(t *testing.T) {
	match := &models.UserCompanyMatch{MatchScore: 72.5, MatchedAxisCount: 8, TechnicalMatch: 80}
	raw := buildScoreSnapshotDocument(map[string]int{"技術志向": 60}, match, time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC))
	var doc scoreSnapshotDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != 2 || doc.LogicVersion != snapshotLogicVersion || doc.ChatWeight != 0.5 {
		t.Fatalf("%+v", doc)
	}
	if doc.Categories["技術志向"] != 60 || doc.MatchScore != 72.5 || doc.MatchedAxisCount != 8 || doc.AxisMatch["技術志向"] != 80 {
		t.Fatalf("%+v", doc)
	}
	if doc.CapturedAt != "2026-11-19T08:00:00Z" {
		t.Fatalf("captured_at = %s", doc.CapturedAt)
	}
}

func TestBuildScoreSnapshotDocument_材料が無ければ空(t *testing.T) {
	if buildScoreSnapshotDocument(nil, nil, time.Now()) != "" {
		t.Fatal("空のスナップショットを残している")
	}
}
