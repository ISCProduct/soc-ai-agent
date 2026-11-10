package prompts

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"

	"Backend/internal/usagectx"
)

// Stage はプロンプト版を記録する工程を表す。
type Stage string

const (
	// Stage は usagectx.Feature とは別の分析軸。1つの工程が複数の機能に対応する場合がある。
	StageQuestionGeneration Stage = "question_generation"
	StageScoreEvaluation    Stage = "score_evaluation"
	StageMatchingReason     Stage = "matching_reason"
	StageCompanyExtraction  Stage = "company_extraction"
)

const defaultPromptVersion = "v2"

var (
	promptVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	warnedPromptSettings sync.Map
)

// ResolvePromptVersion は工程別設定、共通設定、既定値の順に版ラベルを解決する。
//
// ラベルはデプロイされた本文の識別子であり、本文自体はコードで管理する。
func ResolvePromptVersion(stage Stage) string {
	return promptVersionForStage(stage)
}

// WithPromptVersion は解決した版を利用量記録用 context に載せる。
func WithPromptVersion(ctx context.Context, stage Stage) (context.Context, string) {
	version := ResolvePromptVersion(stage)
	return usagectx.WithPromptVersion(ctx, string(stage), version), version
}

func promptVersionForStage(stage Stage) string {
	if !knownPromptStage(stage) {
		warnPromptVersionOnce(stage, "unknown stage")
		return defaultPromptVersion
	}

	stageVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION_" + strings.ToUpper(string(stage))))
	if stageVersion != "" {
		if ValidPromptVersion(stageVersion) {
			return stageVersion
		}
		warnPromptVersionOnce(stage, "invalid stage version")
	}
	globalVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION"))
	if globalVersion != "" {
		if ValidPromptVersion(globalVersion) {
			return globalVersion
		}
		warnPromptVersionOnce(stage, "invalid global version")
	}
	return defaultPromptVersion
}

// ValidPromptVersion は版ラベルとして保存できる形式かを返す。
func ValidPromptVersion(version string) bool {
	return promptVersionPattern.MatchString(version)
}

// FormatMatchingReasonPromptVersion はプロンプト本文に表示する工程付き版名を返す。
func FormatMatchingReasonPromptVersion(version string) string {
	return "matching_reason_" + version
}

func knownPromptStage(stage Stage) bool {
	switch stage {
	case StageQuestionGeneration, StageScoreEvaluation, StageMatchingReason, StageCompanyExtraction:
		return true
	default:
		return false
	}
}

func warnPromptVersionOnce(stage Stage, reason string) {
	key := string(stage) + ":" + reason + ":" + strings.TrimSpace(os.Getenv("PROMPT_VERSION_"+strings.ToUpper(string(stage))))
	if _, loaded := warnedPromptSettings.LoadOrStore(key, struct{}{}); !loaded {
		slog.Warn("プロンプト版設定を無視します", "stage", stage, "reason", reason)
	}
}
