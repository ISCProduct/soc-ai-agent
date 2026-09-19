// Package usagectx は AI 利用量の配賦軸（機能名・実行主体）をリクエストスコープで運ぶ（#1294）。
//
// 呼び出し元（サービス層）と記録層（internal/openai）の間には SDK ラッパーが挟まるため、
// 引数で通すと全メソッドのシグネチャを変えることになる。context で運ぶ。
//
// middleware と openai の両方から参照するため、依存を持たない独立パッケージに置く。
// internal/openai から internal/middleware を参照すると
// openai → middleware → repositories → companyfetch → openai で循環する。
package usagectx

import "context"

// FeatureUnknown は機能名が渡されなかった呼び出しの既定値。
// この件数が「まだ計測できていない経路」の指標になる（DesignDoc §8）。
const FeatureUnknown = "unknown"

// 機能名の定数。api_call_logs.feature に入り、機能別のコスト集計軸になる。
// LLM に分類させず、呼び出し元が定数で渡す（DesignDoc §3.5）。
//
// 値を変えると過去データと突き合わせられなくなるため、リネームは避ける。
const (
	FeatureESReview          = "es_review"
	FeatureESRewrite         = "es_rewrite"
	FeatureResumeReview      = "resume_review"
	FeatureChatAnswerEval    = "chat_answer_eval"
	FeatureChatAnswerCheck   = "chat_answer_check"
	FeatureChatSummary       = "chat_summary"
	FeatureChatQuestion      = "chat_question"
	FeatureAnalysisScoring   = "analysis_scoring"
	FeatureInterviewTurn     = "interview_turn"
	FeatureInterviewReport   = "interview_report"
	FeatureInterviewRealtime = "interview_realtime"
	FeatureInterviewSTT      = "interview_stt"
	FeatureInterviewTTS      = "interview_tts"
	FeatureCompanySearch     = "company_search"
	FeatureCompanyParse      = "company_parse"
	FeatureCompanyCrawl      = "company_crawl"
	FeatureCompanyJobFetch   = "company_job_fetch"
	FeatureMatchingReason    = "matching_reason"
	FeatureDiagnosisQuality  = "diagnosis_quality"
	FeatureGitHubSummary     = "github_summary"
	FeatureReleaseNote       = "release_note"
	FeatureEmbedding         = "embedding"
)

type featureKey struct{}
type actorKey struct{}

// actor は実行主体と配賦先。どちらも解決できないことがあるためポインタで持つ。
type actor struct {
	UserID         *uint
	OrganizationID *uint
}

// WithFeature は以降の AI 呼び出しに機能名を付与する。
//
//	ctx = usagectx.WithFeature(ctx, usagectx.FeatureESReview)
func WithFeature(ctx context.Context, feature string) context.Context {
	if ctx == nil || feature == "" {
		return ctx
	}
	return context.WithValue(ctx, featureKey{}, feature)
}

// Feature は機能名を取り出す。未設定なら FeatureUnknown。
func Feature(ctx context.Context) string {
	if ctx == nil {
		return FeatureUnknown
	}
	if f, ok := ctx.Value(featureKey{}).(string); ok && f != "" {
		return f
	}
	return FeatureUnknown
}

// WithActor は実行主体と配賦先を載せる。HTTP 経路では認証ミドルウェアが一度だけ呼ぶ。
// バッチ経路は主体を持たないため、呼ばなければ NULL で記録される。
func WithActor(ctx context.Context, userID, organizationID uint) context.Context {
	if ctx == nil {
		return ctx
	}
	a := actor{}
	if userID > 0 {
		a.UserID = &userID
	}
	if organizationID > 0 {
		a.OrganizationID = &organizationID
	}
	if a.UserID == nil && a.OrganizationID == nil {
		return ctx
	}
	return context.WithValue(ctx, actorKey{}, a)
}

// Actor は実行主体と配賦先を取り出す。未設定なら nil。
func Actor(ctx context.Context) (userID, organizationID *uint) {
	if ctx == nil {
		return nil, nil
	}
	a, ok := ctx.Value(actorKey{}).(actor)
	if !ok {
		return nil, nil
	}
	return a.UserID, a.OrganizationID
}
