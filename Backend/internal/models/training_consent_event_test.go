package models

import (
	"os"
	"regexp"
	"testing"
)

// #2262 のレビュー: 画面（TrainingConsentCard.tsx）が送る文言の版と、サーバーが受け付ける版は同じでなければならない。
// ずれると、今の画面からの保存がすべて 409 になる（同意も取り消しもできない）。
// 文言を変えて片方だけ版を上げた場合に、ここで気づけるようにする。
func TestTrainingConsentTextVersion_MatchesFrontend(t *testing.T) {
	const path = "../../../frontend/components/profile/TrainingConsentCard.tsx"
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("画面のソースを読めない（%s）: %v", path, err)
	}
	m := regexp.MustCompile(`export const TRAINING_CONSENT_TEXT_VERSION = '([^']*)'`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("%s に TRAINING_CONSENT_TEXT_VERSION の定義が無い", path)
	}
	if got := string(m[1]); got != TrainingConsentTextVersion {
		t.Errorf("画面の版 %q とサーバーの版 %q が違う。文言を変えたら両方を同じ版に上げること", got, TrainingConsentTextVersion)
	}
}
