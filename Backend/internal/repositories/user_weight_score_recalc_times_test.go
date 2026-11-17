package repositories

import (
	"testing"
	"time"

	"Backend/internal/models"
)

// #1770 B1/B5: 置換で元の時刻を引き継ぎ、ロールバックはその時刻と突き合わせて「再計算の後の更新」を見分ける。
func TestChangedSinceRecalculation(t *testing.T) {
	t0 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	snapshot := []models.UserWeightScore{
		{WeightCategory: "技術志向", CreatedAt: t0, UpdatedAt: t1}, // 再計算の前にライブ更新を受けていた行
		{WeightCategory: "チャレンジ志向", CreatedAt: t0, UpdatedAt: t0},
	}
	// 置換直後の行（時刻は引き継ぎ、計算版は v1）
	replaced := func() []models.UserWeightScore {
		return []models.UserWeightScore{
			{WeightCategory: "技術志向", CreatedAt: t0, UpdatedAt: t1, CalculationVersion: "v1"},
			{WeightCategory: "チャレンジ志向", CreatedAt: t0, UpdatedAt: t0, CalculationVersion: "v1"},
		}
	}
	if changedSinceRecalculation(replaced(), snapshot, "v1") {
		t.Fatal("置換の直後は変更なし（再計算前の更新 updated_at > created_at を変更とみなさない）")
	}

	later := replaced()
	later[0].UpdatedAt = t1.Add(24 * time.Hour) // 再計算の後にライブ採点で更新
	if !changedSinceRecalculation(later, snapshot, "v1") {
		t.Fatal("再計算の後の更新を見逃した")
	}

	added := append(replaced(), models.UserWeightScore{WeightCategory: "安定志向", CreatedAt: t1.Add(48 * time.Hour), UpdatedAt: t1.Add(48 * time.Hour), CalculationVersion: "v1"})
	if !changedSinceRecalculation(added, snapshot, "v1") {
		t.Fatal("再計算の後に増えた新しいカテゴリの行を見逃した（戻すと消える）")
	}

	other := replaced()
	other[1].CalculationVersion = "legacy"
	if !changedSinceRecalculation(other, snapshot, "v1") {
		t.Fatal("計算版が違う行を見逃した")
	}

	// 再計算で新しく増えたカテゴリは、元の最大の updated_at を引き継ぐので変更なし
	newCat := append(replaced(), models.UserWeightScore{WeightCategory: "安定志向", CreatedAt: t0, UpdatedAt: t1, CalculationVersion: "v1"})
	if changedSinceRecalculation(newCat, snapshot, "v1") {
		t.Fatal("再計算で増えたカテゴリを変更とみなした")
	}
}

func TestRecalculationTimes(t *testing.T) {
	t0 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	rows := []models.UserWeightScore{
		{WeightCategory: "a", CreatedAt: t0.Add(time.Hour), UpdatedAt: t0.Add(2 * time.Hour)},
		{WeightCategory: "b", CreatedAt: t0, UpdatedAt: t0.Add(5 * time.Hour)},
	}
	by, minCreated, maxUpdated := recalculationTimes(rows)
	if len(by) != 2 || !minCreated.Equal(t0) || !maxUpdated.Equal(t0.Add(5*time.Hour)) {
		t.Fatalf("by=%d min=%v max=%v", len(by), minCreated, maxUpdated)
	}
}
