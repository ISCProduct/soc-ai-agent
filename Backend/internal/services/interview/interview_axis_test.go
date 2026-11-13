package interview

import (
	"Backend/internal/models"
	"strings"
	"testing"
)

func profileWith(f func(p *models.CompanyWeightProfile)) *models.CompanyWeightProfile {
	p := &models.CompanyWeightProfile{
		TechnicalOrientation: 50, TeamworkOrientation: 50, LeadershipOrientation: 50,
		CreativityOrientation: 50, StabilityOrientation: 50, GrowthOrientation: 50,
		WorkLifeBalance: 50, ChallengeSeeking: 50, DetailOrientation: 50, CommunicationSkill: 50,
	}
	f(p)
	return p
}

func axisKeys(axes []interviewAxis) []string {
	keys := make([]string, 0, len(axes))
	for _, a := range axes {
		keys = append(keys, a.Key)
	}
	return keys
}

func TestSelectTargetAxes(t *testing.T) {
	tests := []struct {
		name    string
		profile *models.CompanyWeightProfile
		want    []string
	}{
		{"プロファイル無しは軸なし", nil, nil},
		{"全て50は識別できず軸なし", profileWith(func(p *models.CompanyWeightProfile) {}), nil},
		{"差が小さい(最大-最小=19)は軸なし", profileWith(func(p *models.CompanyWeightProfile) {
			p.TeamworkOrientation = 69
			p.DetailOrientation = 50
		}), nil},
		{"差が閾値ちょうど(20)なら選ぶ", profileWith(func(p *models.CompanyWeightProfile) {
			p.TeamworkOrientation = 70
			p.DetailOrientation = 50
		}), []string{"teamwork"}},
		{"全て高く差が小さいなら軸なし", profileWith(func(p *models.CompanyWeightProfile) {
			p.TechnicalOrientation, p.TeamworkOrientation, p.LeadershipOrientation = 80, 80, 80
			p.CreativityOrientation, p.StabilityOrientation, p.GrowthOrientation = 80, 80, 80
			p.WorkLifeBalance, p.ChallengeSeeking, p.DetailOrientation, p.CommunicationSkill = 80, 80, 80, 80
		}), nil},
		{"上位3軸を重視度の降順で選ぶ", profileWith(func(p *models.CompanyWeightProfile) {
			p.TechnicalOrientation = 90
			p.TeamworkOrientation = 85
			p.GrowthOrientation = 80
			p.ChallengeSeeking = 75
			p.DetailOrientation = 20
		}), []string{"technical", "teamwork", "growth"}},
		{"下限(60)未満の軸は選ばない", profileWith(func(p *models.CompanyWeightProfile) {
			p.LeadershipOrientation = 95
			p.CommunicationSkill = 59
			p.DetailOrientation = 10
		}), []string{"leadership"}},
		{"同点は表の順で安定", profileWith(func(p *models.CompanyWeightProfile) {
			p.CreativityOrientation = 80
			p.TechnicalOrientation = 80
			p.DetailOrientation = 10
		}), []string{"technical", "creativity"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := axisKeys(SelectTargetAxes(tt.profile))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("SelectTargetAxes = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildQuestionQueueWithAxes(t *testing.T) {
	axes := []interviewAxis{axisByKey("teamwork"), axisByKey("growth")}
	custom := []models.InterviewCompanyQuestion{{ID: 1, QuestionText: "必須", IsRequired: true, Category: "企業別"}}
	queue := BuildQuestionQueueWithAxes(7, custom, axes, nil)

	var got []string
	for _, q := range queue {
		got = append(got, q.Source+":"+q.Category)
	}
	want := []string{
		"custom:企業別",
		"topic:自己紹介・志望動機",
		"topic:職務経験・実績",
		"axis:" + axisByKey("teamwork").CategoryLabel(),
		"axis:" + axisByKey("growth").CategoryLabel(),
		"topic:逆質問",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("queue = %v\nwant  %v", got, want)
	}
	for i, q := range queue {
		if q.SortOrder != i {
			t.Errorf("SortOrder[%d] = %d", i, q.SortOrder)
		}
		if q.Source == questionSourceAxis {
			if q.TargetAxis == nil || *q.TargetAxis == "" {
				t.Errorf("軸スロットに TargetAxis が無い: %+v", q)
			}
			if strings.TrimSpace(q.QuestionText) == "" {
				t.Errorf("軸スロットに定型質問が無い")
			}
		} else if q.TargetAxis != nil {
			t.Errorf("軸以外に TargetAxis が入っている: %+v", q)
		}
	}
}

func TestBuildQuestionQueueWithThreeAxesReplacesAllMiddleTopics(t *testing.T) {
	axes := []interviewAxis{axisByKey("technical"), axisByKey("teamwork"), axisByKey("growth")}
	queue := BuildQuestionQueueWithAxes(1, nil, axes, nil)
	var topics []string
	for _, q := range queue {
		if q.Source == questionSourceTopic {
			topics = append(topics, q.Category)
		}
	}
	if strings.Join(topics, "|") != "自己紹介・志望動機|逆質問" {
		t.Fatalf("標準トピック = %v", topics)
	}
}

func TestBuildQuestionQueueWithoutAxesIsUnchanged(t *testing.T) {
	custom := []models.InterviewCompanyQuestion{{ID: 1, QuestionText: "必須", IsRequired: true}, {ID: 2, QuestionText: "推奨"}}
	a := BuildQuestionQueue(3, custom, "ヒント")
	b := BuildQuestionQueueWithAxes(3, custom, nil, []string{"ヒント"})
	if len(a) != len(b) {
		t.Fatalf("len %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Source != b[i].Source || a[i].Category != b[i].Category || a[i].QuestionText != b[i].QuestionText {
			t.Errorf("[%d] %+v != %+v", i, a[i], b[i])
		}
	}
	if len(a) != len(custom)+len(interviewTopics) {
		t.Fatalf("標準5トピックの構成が変わっている: %d", len(a))
	}
}

func TestContainsDiscriminatoryQuestion(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"ご家族の構成を教えてください。", true},
		{"ご家族構成について教えてください。", true},
		{"ご両親の職業は何ですか。", true},
		{"ご出身はどちらですか。", true},
		{"出身地を教えてください。", true},
		{"本籍はどちらですか？", true},
		{"国籍を教えていただけますか。", true},
		{"信仰している宗教はありますか。", true},
		{"支持する政党はありますか。", true},
		{"ご自身の政治的思想について教えてください。", true},
		{"あなたの思想・信条を聞かせてください。", true},
		// 誤検出しないこと
		{"チームで成果を出した経験を教えてください。", false},
		{"設計思想を決めるときに意識したことは何ですか。", false},
		{"ご出身の大学ではどのようなゼミに所属していましたか。", false},
		{"出身学部で学んだことを教えてください。", false},
		{"宗教学のゼミで取り組んだ研究について教えてください。", false},
		{"家族のようにチームで助け合った経験はありますか。", false},
		{"開発チームの構成を変えたときの判断を教えてください。", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ContainsDiscriminatoryQuestion(tt.text); got != tt.want {
			t.Errorf("ContainsDiscriminatoryQuestion(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
