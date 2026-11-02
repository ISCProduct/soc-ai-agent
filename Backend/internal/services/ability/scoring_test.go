package ability

import (
	"reflect"
	"testing"

	"Backend/internal/models"
)

func q(id uint, section string, correct int) models.AbilityQuestion {
	return models.AbilityQuestion{
		ID: id, Section: section, CorrectChoice: correct,
		Choices: `["a","b","c","d"]`,
	}
}

// 採点はサーバだけが行う。無回答は誤答扱いだが出題数には数える
// （飛ばした問を分母から外すと、解かなかった学生の正答率が上がる）。
func TestScoreAnswers(t *testing.T) {
	qs := []models.AbilityQuestion{
		q(1, models.AbilitySectionVerbal, 0),
		q(2, models.AbilitySectionVerbal, 1),
		q(3, models.AbilitySectionNumerical, 2),
		q(4, models.AbilitySectionNumerical, 3),
	}
	tests := []struct {
		name    string
		answers map[uint]int
		want    map[string]SectionScore
	}{
		{
			"全問正解",
			map[uint]int{1: 0, 2: 1, 3: 2, 4: 3},
			map[string]SectionScore{"verbal": {2, 2}, "numerical": {2, 2}},
		},
		{
			"section ごとに別々に数える",
			map[uint]int{1: 0, 2: 0, 3: 2, 4: 2},
			map[string]SectionScore{"verbal": {1, 2}, "numerical": {1, 2}},
		},
		{
			"無回答は誤答だが出題数には数える",
			map[uint]int{1: 0},
			map[string]SectionScore{"verbal": {1, 2}, "numerical": {0, 2}},
		},
		{
			"一問も答えていなくても出題数は出す",
			map[uint]int{},
			map[string]SectionScore{"verbal": {0, 2}, "numerical": {0, 2}},
		},
		{
			"選択肢の範囲外は誤答",
			map[uint]int{1: 99, 2: -1, 3: 2, 4: 3},
			map[string]SectionScore{"verbal": {0, 2}, "numerical": {2, 2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScoreAnswers(qs, tt.answers)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// 出題が無い section は返さない。0/0 を見せると「受けたのに全問不正解」と読める。
func TestScoreAnswersOmitsEmptySection(t *testing.T) {
	got, err := ScoreAnswers([]models.AbilityQuestion{q(1, models.AbilitySectionVerbal, 0)}, map[uint]int{1: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got[models.AbilitySectionNumerical]; ok {
		t.Errorf("出題の無い section を返した: %v", got)
	}
	if got[models.AbilitySectionVerbal] != (SectionScore{1, 1}) {
		t.Errorf("verbal = %v", got[models.AbilitySectionVerbal])
	}
}

// section が正典外の問があればエラーにする。黙って別枠に数えると結果の意味が変わる。
func TestScoreAnswersRejectsUnknownSection(t *testing.T) {
	_, err := ScoreAnswers([]models.AbilityQuestion{q(1, "english", 0)}, map[uint]int{1: 0})
	if err == nil {
		t.Fatal("正典外の section を受け入れた")
	}
}

func TestValidChoice(t *testing.T) {
	question := q(1, models.AbilitySectionVerbal, 0)
	tests := []struct {
		choice int
		want   bool
	}{{0, true}, {3, true}, {4, false}, {-1, false}}
	for _, tt := range tests {
		got, err := ValidChoice(question, tt.choice)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("ValidChoice(%d) = %v, want %v", tt.choice, got, tt.want)
		}
	}
}

func TestChoicesOfRejectsBrokenJSON(t *testing.T) {
	if _, err := ChoicesOf(models.AbilityQuestion{ID: 1, Choices: `[`}); err == nil {
		t.Fatal("壊れた選択肢を受け入れた")
	}
}
