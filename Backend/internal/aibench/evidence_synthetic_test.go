package aibench

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/interview"
)

// SOCAIAGENT-675: 根拠照合（interview.ValidateEvidence）の検出率と誤検知率を、合成ケースで測る。
//
// 面接ターゲットの通常の計測（evidence_not_spoken）は LLM が書いた根拠を数えるので、
// 照合を変えた効果とモデルの揺れが混ざる。ここでは「発話」と「根拠」を固定した合成ケースで、
// 照合だけの数字を出す（LLM は呼ばない）。
//
//	検出率   = 捏造（fabricated）のうち未照合にできた割合
//	誤検知率 = 正当（legit）のうち未照合にしてしまった割合
//
// 実測（docs/research/ai-eval/interview-evidence-synthetic.jsonl、正当12件・捏造10件）:
//
//	                        検出率          誤検知率
//	追加前（算用数字だけ）       2/10 (20%)     4/12 (33%)
//	追加後（漢数字・固有名詞）  10/10 (100%)    1/12 (8%)
//
// 追加前の誤検知4件は、発話が漢数字で根拠が算用数字のもの。見逃し8件は、漢数字の数値の
// 入れ替えと、カタカナ・英字の語の入れ替え。
//
// 追加後に残る誤検知は、カタカナ語への言い換え（意思疎通→コミュニケーション）の1件。
// カタカナ語を全部対象にした結果で、除外リストの要否はこの数字と本番の計測で判断する（U6）。
// 長音の表記ゆれは、根拠が短い側（サーバー→サーバ）なら発話に含まれるので通るが、
// 逆（サーバ→サーバー）は誤検知になる。下の下限・上限は追加後の実測で、照合を変えて悪化したら落ちる。
const (
	syntheticMinDetection     = 10
	syntheticMaxFalsePositive = 1
)

type syntheticEvidenceCase struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Spoken   string `json:"spoken"`
	Evidence string `json:"evidence"`
	Note     string `json:"note"`
}

func Test合成ケースで根拠照合の検出率と誤検知率を測る(t *testing.T) {
	path := "../../../docs/research/ai-eval/interview-evidence-synthetic.jsonl"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s の読み込みに失敗: %v", path, err)
	}
	var detected, fabricated, falsePositive, legit int
	for line := range strings.SplitSeq(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var c syntheticEvidenceCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatalf("解析に失敗: %v: %s", err, line)
		}
		if strings.TrimSpace(c.Note) == "" {
			t.Errorf("%s: note（ラベルの理由）が空", c.ID)
		}
		spoken := interview.SpokenContent([]models.InterviewUtterance{{Role: "user", Text: c.Spoken}})
		unmatched := len(interview.ValidateEvidence(map[string]string{"logic": c.Evidence}, spoken).Unmatched) > 0
		switch c.Label {
		case "fabricated":
			fabricated++
			if unmatched {
				detected++
			} else {
				t.Logf("見逃し %s: %s", c.ID, c.Note)
			}
		case "legit":
			legit++
			if unmatched {
				falsePositive++
				t.Logf("誤検知 %s: %s", c.ID, c.Note)
			}
		default:
			t.Fatalf("%s: label = %q（legit / fabricated）", c.ID, c.Label)
		}
	}
	t.Logf("検出率 %d/%d、誤検知率 %d/%d", detected, fabricated, falsePositive, legit)
	if fabricated == 0 || legit == 0 {
		t.Fatal("正当・捏造の両方のケースが要る")
	}
	if detected < syntheticMinDetection {
		t.Errorf("検出 %d 件、下限 %d 件を下回った", detected, syntheticMinDetection)
	}
	if falsePositive > syntheticMaxFalsePositive {
		t.Errorf("誤検知 %d 件、上限 %d 件を超えた", falsePositive, syntheticMaxFalsePositive)
	}
}
