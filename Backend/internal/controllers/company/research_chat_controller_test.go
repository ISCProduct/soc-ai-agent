package company

import (
	"strings"
	"testing"
)

// 質問は外部の AI へ送る前に伏字にする（SOCAIAGENT-724）。値は合成データ。
func TestResearchChatController_MasksQuestion(t *testing.T) {
	ctrl := NewResearchChatController(nil, nil)
	got, err := ctrl.mask(1, "連絡先は taro.example@example.com と 090-1234-5678 です。この会社の事業は？")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "taro.example@example.com") || strings.Contains(got, "090-1234-5678") {
		t.Errorf("メールアドレスや電話番号が伏字になっていない: %q", got)
	}
	if !strings.Contains(got, "この会社の事業は？") {
		t.Errorf("質問の本文は残す: %q", got)
	}
}
