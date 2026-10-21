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
	// Stage は usagectx.Feature とは別の分析軸。question_generation は chat_question、
	// score_evaluation は chat_answer_eval、matching_reason は matching_reason、
	// company_extraction は company_search / company_parse のAI呼び出しに対応する。
	StageQuestionGeneration Stage = "question_generation"
	StageScoreEvaluation    Stage = "score_evaluation"
	StageMatchingReason     Stage = "matching_reason"
	StageCompanyExtraction  Stage = "company_extraction"
)

// VersionAssignment は工程に割り当てられた版とA/B群を表す。
type VersionAssignment struct {
	Version string
	Variant string
}

var (
	promptVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	warnedPromptSettings sync.Map
)

// ResolvePromptVersion は工程別設定と実装済みの本文からプロンプト版を決める。
//
// 候補本文が実装されるまでは、全ての利用者に実在するcontrol版のみを割り当てる。
func ResolvePromptVersion(stage Stage, _ uint) VersionAssignment {
	if strings.TrimSpace(os.Getenv("PROMPT_AB_"+strings.ToUpper(string(stage)))) != "" {
		warnPromptVersionOnce(stage, "A/B assignment requires a registered candidate prompt")
	}
	return VersionAssignment{Version: promptVersionForStage(stage), Variant: "control"}
}

// WithPromptVersion は選択した版を利用量記録用 context に載せる。
func WithPromptVersion(ctx context.Context, stage Stage, subjectID uint) (context.Context, VersionAssignment) {
	assignment := ResolvePromptVersion(stage, subjectID)
	return usagectx.WithPromptVersion(ctx, string(stage), assignment.Version, assignment.Variant), assignment
}

func promptVersionForStage(stage Stage) string {
	if !hasVersionedPrompt(stage, "v2") {
		warnPromptVersionOnce(stage, "unknown stage")
		return "v2"
	}

	stageVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION_" + strings.ToUpper(string(stage))))
	if stageVersion != "" {
		if validPromptVersion(stageVersion) && hasVersionedPrompt(stage, stageVersion) {
			return stageVersion
		}
		warnPromptVersionOnce(stage, "unregistered stage version")
	}

	globalVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION"))
	if globalVersion == "" {
		globalVersion = strings.TrimSpace(PromptVersion)
	}
	if globalVersion != "" {
		if validPromptVersion(globalVersion) && hasVersionedPrompt(stage, globalVersion) {
			return globalVersion
		}
		warnPromptVersionOnce(stage, "unregistered global version")
	}
	return "v2"
}

func validPromptVersion(version string) bool {
	return promptVersionPattern.MatchString(version)
}

// FormatMatchingReasonPromptVersion はプロンプト本文に表示する工程付き版名を返す。
func FormatMatchingReasonPromptVersion(version string) string {
	return "matching_reason_" + version
}

// hasVersionedPrompt は、工程ごとに本文が存在する版だけを許可する。
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

func warnPromptVersionOnce(stage Stage, reason string) {
	key := string(stage) + ":" + reason + ":" + strings.TrimSpace(os.Getenv("PROMPT_VERSION_"+strings.ToUpper(string(stage))))
	if _, loaded := warnedPromptSettings.LoadOrStore(key, struct{}{}); !loaded {
		slog.Warn("プロンプト版設定を無視します", "stage", stage, "reason", reason)
	}
}
