package interview

import (
	"strings"
	"testing"
)

// TestBuildInterviewSystemPromptToneGuideline は #910 の回帰テスト。
// AIが叱責的なトーンで応答しないよう、プロンプトに中立トーンの指示が
// 含まれていることを検証する。
func TestBuildInterviewSystemPromptToneGuideline(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"テスト株式会社", "", "エンジニア", "", "general",
		nil, nil, 0, 0, 1, 5, 0, 180, nil,
	)
	if !strings.Contains(prompt, "叱責") {
		t.Fatalf("prompt missing tone guideline (叱責禁止): %s", prompt)
	}
}

func TestBuildInterviewSystemPromptDeepeningMotivationCriteria(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"テスト株式会社", "", "エンジニア", "", "general",
		nil, nil, 0, 0, 1, 5, 0, 180, nil,
	)
	if !strings.Contains(prompt, "きっかけ") || !strings.Contains(prompt, "継続") {
		t.Fatalf("prompt missing motivation/continuity deepening criteria: %s", prompt)
	}
}

func TestBuildInterviewSystemPromptKeepsTTSResponsesShort(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"テスト株式会社", "", "エンジニア", "", "general",
		nil, nil, 0, 0, 1, 5, 0, 180, nil,
	)
	if !strings.Contains(prompt, "150文字以内") {
		t.Fatalf("prompt missing TTS response length limit: %s", prompt)
	}
}

func TestIsEngineerPosition(t *testing.T) {
	tests := []struct {
		name     string
		position string
		want     bool
	}{
		{name: "日本語エンジニア", position: "バックエンドエンジニア", want: true},
		{name: "英語小文字", position: "software engineer", want: true},
		{name: "英語大文字", position: "Senior Developer", want: true},
		{name: "SRE", position: "Site Reliability Engineer (SRE)", want: true},
		{name: "非エンジニア", position: "法人営業", want: false},
		{name: "空文字", position: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEngineerPosition(tt.position); got != tt.want {
				t.Fatalf("isEngineerPosition(%q) = %v, want %v", tt.position, got, tt.want)
			}
		})
	}
}

// TestBuildInterviewSystemPromptInterviewerRole は #1223 の回帰テスト。
//
// 面接官が企業側であること・逆質問には自分が答えることは、従来プロンプトに
// 書かれておらずモデルの暗黙の補完に依存していた。#1209 の計測では、
// 補完できないモデルに与えると自社を「御社」と呼び（8ターン中5回）、
// 逆質問には答えず聞き返す。逆質問は面接の最後のトピックなので、
// そこで毎回面接が壊れる。
func TestBuildInterviewSystemPromptInterviewerRole(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"テスト株式会社", "", "エンジニア", "研修制度あり", "general",
		nil, nil, 0, 0, 1, 5, 0, 180, nil,
	)

	tests := []struct {
		name string
		want string
		why  string
	}{
		{"企業側であることの明示", "採用する側", "応募者と取り違えると逆質問で役割が反転する"},
		{"自社の呼び方", "弊社", "自社を「御社」と呼ぶ誤りが実測で発生していた"},
		{"御社を使わない指示", "「御社」は応募者が使う言葉", "禁止を明示しないと補完できないモデルが誤用する"},
		{"逆質問に答える指示", "逆質問", "面接の最後のトピック。破綻すると毎回そこで終わる"},
		{"質問で返さない指示", "質問で返さない", "聞き返しは実測で発生した具体的な失敗"},
		{"脱線時の扱い", "面接のトピックへ戻す", "雑談に引きずられると面接が乗っ取られる"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(prompt, tt.want) {
				t.Errorf("プロンプトに %q が無い（%s）", tt.want, tt.why)
			}
		})
	}
}

// 企業情報が無い構成でも役割の指示は落ちてはいけない。
// 企業未選択の面接でも逆質問は行われる。
func TestBuildInterviewSystemPromptRoleSurvivesWithoutCompanyInfo(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"", "", "", "", "general",
		nil, nil, 0, 0, 1, 5, 0, 180, nil,
	)
	for _, want := range []string{"採用する側", "弊社", "質問で返さない"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("企業情報が無いと %q が落ちている", want)
		}
	}
}
