package chat

import (
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
)

type judgmentProgressRepo struct {
	repository.UserAnalysisProgressRepository
	list []models.UserAnalysisProgress
}

func (r judgmentProgressRepo) FindByUserAndSession(uint, string) ([]models.UserAnalysisProgress, error) {
	return r.list, nil
}

type judgmentPhaseRepo struct {
	repository.AnalysisPhaseRepository
	list []models.AnalysisPhase
}

func (r judgmentPhaseRepo) FindAll() ([]models.AnalysisPhase, error) { return r.list, nil }

type judgmentScoreRepo struct {
	repository.UserWeightScoreRepository
	list []models.UserWeightScore
}

func (r judgmentScoreRepo) FindByUserAndSession(uint, string) ([]models.UserWeightScore, error) {
	return r.list, nil
}

func sc(cat string, v int) models.UserWeightScore {
	return models.UserWeightScore{WeightCategory: cat, Score: float64(v)}
}

// 合成データ。フェーズは職種4・興味4・適性4・将来3（合計15問）。
func newJudgmentService(valid []int, scores ...models.UserWeightScore) *ChatService {
	phases := []models.AnalysisPhase{
		{ID: 1, PhaseName: "job_analysis"}, {ID: 2, PhaseName: "interest_analysis"},
		{ID: 3, PhaseName: "aptitude_analysis"}, {ID: 4, PhaseName: "future_analysis"},
	}
	var progresses []models.UserAnalysisProgress
	for i, v := range valid {
		progresses = append(progresses, models.UserAnalysisProgress{PhaseID: uint(i + 1), ValidAnswers: v})
	}
	return &ChatService{
		progressRepo:        judgmentProgressRepo{list: progresses},
		phaseRepo:           judgmentPhaseRepo{list: phases},
		userWeightScoreRepo: judgmentScoreRepo{list: scores},
	}
}

func TestJudgmentFor(t *testing.T) {
	spread := []models.UserWeightScore{sc("a", 80), sc("b", 30), sc("c", 55), sc("d", 60), sc("e", 45), sc("f", 70)}
	flat := []models.UserWeightScore{sc("a", 50), sc("b", 51), sc("c", 50), sc("d", 52), sc("e", 50), sc("f", 51)}

	t.Run("15問に満たなくても、傾向が出ていれば完了（職種1+興味2+適性3=6問）", func(t *testing.T) {
		got := newJudgmentService([]int{4, 2, 3}, spread...).judgmentFor(1, "s")
		if !got.Ready || got.Percent != 100 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("全フェーズを終えて15問答えても、傾向が出ていなければ完了にしない", func(t *testing.T) {
		got := newJudgmentService([]int{4, 4, 4, 3}, flat...).judgmentFor(1, "s")
		if got.Ready || got.Percent >= 100 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("床（6問）に満たなければ、傾向が出ていても完了にしない", func(t *testing.T) {
		got := newJudgmentService([]int{4, 2, 2}, spread...).judgmentFor(1, "s")
		if got.Ready {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("4軸しか評価できていなければ、傾向が出ていても完了にしない（推薦の暫定表示の下限は超えているが、6軸に満たない）", func(t *testing.T) {
		four := spread[:4]
		got := newJudgmentService([]int{4, 4, 4}, four...).judgmentFor(1, "s")
		if got.Ready || got.Percent >= 100 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("読めないときは、完了にしない（ゼロ値）", func(t *testing.T) {
		if got := (&ChatService{}).judgmentFor(1, "s"); got.Ready || got.Percent != 0 {
			t.Fatalf("%+v", got)
		}
	})
}
