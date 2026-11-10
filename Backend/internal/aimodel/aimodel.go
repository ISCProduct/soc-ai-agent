// Package aimodel は工程ごとの LLM のモデルを決める（#1526）。
//
// 工程を「学生に見える判断（採点・面接応答・レポート・マッチング理由）」と
// 「内部の定型処理（抽出・parse・要約）」の2層に分け、層ごとにモデルを差し替えられるようにする。
// 全体の単価を上げずに配分を変えるのが狙い（#1124 / #1131 のコスト方針）。
//
// モデルを決める順番:
//  1. 工程ごとの環境変数（既存の名前をそのまま使う。例 OPENAI_INTERVIEW_MODEL）
//  2. 層ごとの環境変数（AI_MODEL_PREMIUM / AI_MODEL_LIGHT）
//  3. 今までの既定（空文字なら呼び出し側の既定 = AI_TEXT_MODEL → OPENAI_MODEL → gpt-4o-mini）
//
// 何も設定しなければ今までと同じモデルになるので、導入だけではコストは変わらない。
// 層の切り替えは aibench（docs/research/ai-eval）で品質とコストを測ってから行う。
package aimodel

import (
	"os"
	"slices"
	"strings"
)

// Tier はモデルの層。
type Tier string

const (
	// Premium は学生に見える判断に使う上位の層。
	Premium Tier = "premium"
	// Light は内部の定型処理に使う軽量の層。
	Light Tier = "light"
)

// 層ごとの環境変数。
const (
	EnvPremium = "AI_MODEL_PREMIUM"
	EnvLight   = "AI_MODEL_LIGHT"
)

// Stage はモデルを決める単位の工程。
type Stage string

const (
	StageInterviewResponse Stage = "interview_response"
	StageInterviewReport   Stage = "interview_report"
	StageScoreEvaluation   Stage = "score_evaluation"
	StageMatchingReason    Stage = "matching_reason"
	StageCompanyExtract    Stage = "company_extract"
	StageCompanyParse      Stage = "company_parse"
	StageChatSummary       Stage = "chat_summary"
)

type stageConfig struct {
	tier Tier
	// env は工程ごとの環境変数。既存の名前を残し、設定済みの環境の挙動を変えない。
	env string
	// fallback は今までの既定。空なら呼び出し側の既定（クライアントの DefaultModel）。
	fallback string
}

var stages = map[Stage]stageConfig{
	// 学生に見える判断
	StageInterviewResponse: {tier: Premium, env: "OPENAI_INTERVIEW_MODEL", fallback: "gpt-4o-mini"},
	StageInterviewReport:   {tier: Premium, env: "INTERVIEW_REPORT_MODEL"},
	StageScoreEvaluation:   {tier: Premium, env: "AI_MODEL_SCORE_EVALUATION"},
	StageMatchingReason:    {tier: Premium, env: "AI_MODEL_MATCHING_REASON"},
	// 内部の定型処理
	StageCompanyExtract: {tier: Light, env: "OPENAI_COMPANY_EXTRACT_MODEL", fallback: "gpt-4o-mini"},
	StageCompanyParse:   {tier: Light, env: "OPENAI_COMPANY_PARSE_MODEL", fallback: "gpt-4o-mini"},
	StageChatSummary:    {tier: Light, env: "AI_MODEL_CHAT_SUMMARY"},
}

// For は工程のモデルを返す。空文字は「呼び出し側の既定を使う」を表す。
// 未知の工程は空文字を返す（呼び出し側の既定のまま）。
func For(stage Stage) string {
	cfg, ok := stages[stage]
	if !ok {
		return ""
	}
	if v := strings.TrimSpace(os.Getenv(cfg.env)); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(tierEnv(cfg.tier))); v != "" {
		return v
	}
	return cfg.fallback
}

// TierOf は工程の層を返す。
func TierOf(stage Stage) Tier { return stages[stage].tier }

// EnvOf は工程ごとの環境変数の名前を返す。
func EnvOf(stage Stage) string { return stages[stage].env }

// Stages は登録済みの工程を名前順で返す（文書とテスト用）。
func Stages() []Stage {
	out := make([]Stage, 0, len(stages))
	for s := range stages {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

func tierEnv(t Tier) string {
	if t == Premium {
		return EnvPremium
	}
	return EnvLight
}
