package prompts

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// PromptVersion はプロンプトバージョンを管理する。
// 環境変数 PROMPT_VERSION で上書き可能（デフォルト: "v1"）。
var PromptVersion = func() string {
	if v := os.Getenv("PROMPT_VERSION"); v != "" {
		return v
	}
	return "v1"
}()

// JobType は職種タイプを表す。
type JobType string

const (
	JobTypeEngineer  JobType = "エンジニア"
	JobTypeSales     JobType = "営業"
	JobTypeMarketing JobType = "マーケ"
	JobTypeDesign    JobType = "デザイン"
	JobTypeGeneral   JobType = "一般"
)

// JobTypeConfig は職種ごとのプロンプト設定を保持する。
type JobTypeConfig struct {
	Type          JobType
	ToneGuideline string // 質問トーンの方向性
	TechFocusNote string // 技術フォーカスの注記
}

var jobTypeConfigs = map[JobType]JobTypeConfig{
	JobTypeEngineer: {
		Type:          JobTypeEngineer,
		ToneGuideline: "技術的な話題に深く踏み込んでよい。コードやツールの具体的な話を歓迎する。",
		TechFocusNote: "プログラミング・システム設計・ツール活用について具体的に聞く。",
	},
	JobTypeSales: {
		Type:          JobTypeSales,
		ToneGuideline: "対人スキル・交渉力・コミュニケーションを重視した文脈で質問する。",
		TechFocusNote: "ITツール活用や業務効率化への関心を聞く。プログラミング経験は前提としない。",
	},
	JobTypeMarketing: {
		Type:          JobTypeMarketing,
		ToneGuideline: "創造性・データ活用・マーケット感覚を重視した文脈で質問する。",
		TechFocusNote: "デジタルマーケティングツールやSNS活用への関心を聞く。",
	},
	JobTypeDesign: {
		Type:          JobTypeDesign,
		ToneGuideline: "ビジュアル表現・ユーザー体験・創造的思考を重視した文脈で質問する。",
		TechFocusNote: "デザインツールやUI/UXへの感度を聞く。",
	},
	JobTypeGeneral: {
		Type:          JobTypeGeneral,
		ToneGuideline: "業種に依存しない汎用的な文脈で質問する。",
		TechFocusNote: "ITツールへの親しみやすさや効率化への意識を聞く。",
	},
}

// InferJobType は職種名文字列から JobType を推定する。
func InferJobType(jobCategoryName string) JobType {
	switch {
	case strings.Contains(jobCategoryName, "エンジニア") ||
		strings.Contains(jobCategoryName, "開発") ||
		strings.Contains(jobCategoryName, "プログラマ") ||
		strings.Contains(jobCategoryName, "SE") ||
		strings.Contains(jobCategoryName, "IT"):
		return JobTypeEngineer
	case strings.Contains(jobCategoryName, "営業") ||
		strings.Contains(jobCategoryName, "セールス"):
		return JobTypeSales
	case strings.Contains(jobCategoryName, "マーケ") ||
		strings.Contains(jobCategoryName, "広告") ||
		strings.Contains(jobCategoryName, "PR"):
		return JobTypeMarketing
	case strings.Contains(jobCategoryName, "デザイン") ||
		strings.Contains(jobCategoryName, "デザイナー") ||
		strings.Contains(jobCategoryName, "UI") ||
		strings.Contains(jobCategoryName, "UX"):
		return JobTypeDesign
	default:
		return JobTypeGeneral
	}
}

// GetJobTypeConfig は職種名から設定を返し、使用バージョンをログに記録する。
func GetJobTypeConfig(jobCategoryName string) JobTypeConfig {
	jt := InferJobType(jobCategoryName)
	cfg, ok := jobTypeConfigs[jt]
	if !ok {
		cfg = jobTypeConfigs[JobTypeGeneral]
	}
	slog.Info("プロンプト設定取得",
		"job_type", string(jt),
		"prompt_version", PromptVersion,
	)
	return cfg
}

// BuildJobTypeToneGuidance は職種別トーンガイダンスセクションを返す。
func BuildJobTypeToneGuidance(jobCategoryName string) string {
	cfg := GetJobTypeConfig(jobCategoryName)
	return fmt.Sprintf("## 職種別トーンガイドライン（%s）\n- %s\n- %s",
		string(cfg.Type), cfg.ToneGuideline, cfg.TechFocusNote)
}

// LogPromptVersion は使用するプロンプトバージョンをログに記録する。
func LogPromptVersion(context string) {
	slog.Info("プロンプトバージョン使用",
		"context", context,
		"version", PromptVersion,
	)
}
