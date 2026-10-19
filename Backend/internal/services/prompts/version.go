package prompts

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

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

// VersionAssignment は工程に割り当てられた版とA/B群を表す。
type VersionAssignment struct {
	Version string
	Variant string
}

// ResolvePromptVersion は工程別設定と安定した利用者IDからプロンプト版を決める。
//
// PROMPT_AB_<STAGE> は "candidate:percent" 形式で指定する。candidate に割り当たら
// なかった利用者には工程別版、次に従来の PROMPT_VERSION が適用される。
func ResolvePromptVersion(stage Stage, subjectID uint) VersionAssignment {
	base := promptVersionForStage(stage)
	if !promptVersionFitsStorage(stage, base) {
		slog.Warn("プロンプト版設定を無視します", "stage", stage, "reason", "version too long")
		base = "v2"
	}
	stageName := strings.ToUpper(string(stage))
	experiment := strings.TrimSpace(os.Getenv("PROMPT_AB_" + stageName))
	if experiment == "" {
		return VersionAssignment{Version: base, Variant: "control"}
	}

	parts := strings.Split(experiment, ":")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		slog.Warn("プロンプトA/B設定を無視します", "stage", stage, "reason", "invalid format")
		return VersionAssignment{Version: base, Variant: "control"}
	}
	percent, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || percent < 0 || percent > 100 {
		slog.Warn("プロンプトA/B設定を無視します", "stage", stage, "reason", "invalid percent")
		return VersionAssignment{Version: base, Variant: "control"}
	}
	if percent == 0 || subjectID == 0 {
		return VersionAssignment{Version: base, Variant: "control"}
	}
	candidate := strings.TrimSpace(parts[0])
	if !promptVersionFitsStorage(stage, candidate) {
		slog.Warn("プロンプトA/B設定を無視します", "stage", stage, "reason", "candidate version too long")
		return VersionAssignment{Version: base, Variant: "control"}
	}
	if candidate == base {
		slog.Warn("プロンプトA/B設定を無視します", "stage", stage, "reason", "candidate and control versions are identical")
		return VersionAssignment{Version: base, Variant: "control"}
	}
	if !hasVersionedPrompt(stage, base) || !hasVersionedPrompt(stage, candidate) {
		slog.Warn("プロンプトA/B設定を無視します", "stage", stage, "reason", "candidate prompt is not registered")
		return VersionAssignment{Version: base, Variant: "control"}
	}

	bucket := promptVersionBucket(stage, subjectID)
	if bucket < percent {
		return VersionAssignment{Version: candidate, Variant: "candidate"}
	}
	return VersionAssignment{Version: base, Variant: "control"}
}

// WithPromptVersion は選択した版を利用量記録用 context に載せる。
func WithPromptVersion(ctx context.Context, stage Stage, subjectID uint) (context.Context, VersionAssignment) {
	assignment := ResolvePromptVersion(stage, subjectID)
	return usagectx.WithPromptVersion(ctx, string(stage), assignment.Version, assignment.Variant), assignment
}

func promptVersionForStage(stage Stage) string {
	stageVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION_" + strings.ToUpper(string(stage))))
	if stageVersion != "" {
		return stageVersion
	}
	if globalVersion := strings.TrimSpace(os.Getenv("PROMPT_VERSION")); globalVersion != "" {
		return globalVersion
	}
	if strings.TrimSpace(PromptVersion) != "" {
		return strings.TrimSpace(PromptVersion)
	}
	return "v2"
}

func promptVersionFitsStorage(stage Stage, version string) bool {
	maxLength := 64
	if stage == StageMatchingReason {
		maxLength -= len("matching_reason_")
	}
	return len(version) <= maxLength
}

// hasVersionedPrompt は、実際に本文を実装している工程・版だけを実験候補として許可する。
// 新しい本文を追加した際は、ここにも工程・版を登録する。
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

func promptVersionBucket(stage Stage, subjectID uint) int {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", stage, subjectID)))
	value := uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3])
	return int(value % 100)
}
