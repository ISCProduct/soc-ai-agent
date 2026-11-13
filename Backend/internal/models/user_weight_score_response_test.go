package models

import (
	"encoding/json"
	"testing"
	"time"

	"Backend/domain/entity"
)

// 応答の形は、移行前に応答へ出していた domain/entity の UserWeightScore と同じでなければならない。
// entity が無くなるまでの間、同じ値を JSON にして突き合わせる。
func TestUserWeightScoreResponse_従来の応答と同じJSON(t *testing.T) {
	at := time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC)
	row := UserWeightScore{
		ID: 7, OrganizationID: 3, UserID: 11, SessionID: "s-1", Source: ScoreSourceAssessment,
		WeightCategory: "技術志向", Score: 82, CalculationVersion: "v1", PromptVersion: "p2",
		CreatedAt: at, UpdatedAt: at.Add(time.Hour),
	}
	old := entity.UserWeightScore{
		ID: 7, UserID: 11, SessionID: "s-1", WeightCategory: "技術志向", Score: 82,
		CalculationVersion: "v1", PromptVersion: "p2", CreatedAt: at, UpdatedAt: at.Add(time.Hour),
	}
	got, err := json.Marshal(NewUserWeightScoreResponses([]UserWeightScore{row}))
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal([]entity.UserWeightScore{old})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("応答の JSON が従来と違う\n got: %s\nwant: %s", got, want)
	}
}

// キー名は api/openapi.yaml の UserWeightScore と同じ。OrganizationID と Source は出さない。
func TestUserWeightScoreResponse_キー名(t *testing.T) {
	b, _ := json.Marshal(NewUserWeightScoreResponses([]UserWeightScore{{ID: 1, OrganizationID: 9, Source: "assessment"}}))
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	want := []string{"ID", "UserID", "SessionID", "WeightCategory", "Score", "CalculationVersion", "PromptVersion", "CreatedAt", "UpdatedAt"}
	if len(rows) != 1 || len(rows[0]) != len(want) {
		t.Fatalf("キーの数が違う: %v", rows)
	}
	for _, k := range want {
		if _, ok := rows[0][k]; !ok {
			t.Errorf("キー %s が無い: %s", k, b)
		}
	}
}

// nil は null、空は [] のまま渡す（従来の応答の形を変えない）。
func TestNewUserWeightScoreResponses_nilと空を区別する(t *testing.T) {
	if b, _ := json.Marshal(NewUserWeightScoreResponses(nil)); string(b) != "null" {
		t.Errorf("nil: %s", b)
	}
	if b, _ := json.Marshal(NewUserWeightScoreResponses([]UserWeightScore{})); string(b) != "[]" {
		t.Errorf("空: %s", b)
	}
}
