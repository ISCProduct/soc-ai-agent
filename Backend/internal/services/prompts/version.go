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

// Stage は独立して切り替え可能なプロンプト工程を表す。
type Stage string

const (
	StageQuestionGeneration Stage = "question_generation"
	StageScoreEvaluation    Stage = "score_evaluation"
	StageMatchingReason     Stage = "matching_reason"
	StageCompanyExtraction  Stage = "company_extraction"
)

var (
	promptVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	warnedPromptSettings sync.Map
)

// VersionAssignment は工程に割り当てられた版とA/B群を表す。
type VersionAssignment struct {
	Version string
	Variant string
}

// ResolvePromptVersion は工程別設定と実装済みの本文からプロンプト版を決める。
//
// 現在は v2 の本文のみ実装されているため、未登録版と A/B 割当は受け付けない。
func ResolvePromptVersion(stage Stage, _ uint) VersionAssignment {
	base := promptVersionForStage(stage)
	stageName := strings.ToUpper(string(stage))
	experiment := strings.TrimSpace(os.Getenv("PROMPT_AB_" + stageName))
	if experiment != "" {
		warnPromptSettingOnce(stage, "A/B assignment requires a registered candidate prompt")
	}
	return VersionAssignment{Version: base, Variant: "control"}
}

// WithPromptVersion は選択した版を利用量記録用 context に載せる。
func WithPromptVersion(ctx context.Context, stage Stage, subjectID uint) (context.Context, VersionAssignment) {
	assignment := ResolvePromptVersion(stage, subjectID)
	return usagectx.WithPromptVersion(ctx, string(stage), assignment.Version, assignment.Variant), assignment
}

func promptVersionForStage(stage Stage) string {
	if !hasVersionedPrompt(stage, "v2") {
		warnPromptSettingOnce(stage, "unknown stage")
		return "v2"
	}

	stageVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION_" + strings.ToUpper(string(stage))))
	if stageVersion != "" {
		if validPromptVersion(stageVersion) && hasVersionedPrompt(stage, stageVersion) {
			return stageVersion
		}
		warnPromptSettingOnce(stage, "unregistered stage version")
	}
	if globalVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION")); globalVersion != "" {
		if validPromptVersion(globalVersion) && hasVersionedPrompt(stage, globalVersion) {
			return globalVersion
		}
		warnPromptSettingOnce(stage, "unregistered global version")
	}
	if globalVersion := strings.TrimSpace(PromptVersion); globalVersion != "" {
		if validPromptVersion(globalVersion) && hasVersionedPrompt(stage, globalVersion) {
			return globalVersion
		}
	}
	return "v2"
}

func validPromptVersion(version string) bool {
	return promptVersionPattern.MatchString(version)
}

func hasVersionedPrompt(stage Stage, version string) bool {
	if version != "v2" {
		return false
	}
	switch stage {
	case StageQuestionGeneration, StageScoreEvaluation, StageMatchingReason, StageCompanyExtraction:
		return true
	default:
		return false
	}
}

func warnPromptSettingOnce(stage Stage, reason string) {
	key := string(stage) + ":" + reason + ":" + strings.TrimSpace(os.Getenv("PROMPT_VERSION_"+strings.ToUpper(string(stage))))
	if _, loaded := warnedPromptSettings.LoadOrStore(key, struct{}{}); !loaded {
		slog.Warn("プロンプト版設定を無視します", "stage", stage, "reason", reason)
	}
}
