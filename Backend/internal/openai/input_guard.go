package openai

import (
	"context"
	"errors"
	"fmt"

	"Backend/internal/usagectx"
)

// ErrInputGuardFailed は、入力ガード（伏字）を作れなかった・かけられなかったことを表す。
// このエラーのときは、外部へ何も送らない（fail-closed）。
var ErrInputGuardFailed = errors.New("入力の伏字を準備できなかったため、外部へ送信しませんでした")

// InputGuard は、外部へ送る前の、学生の文章を伏字にする関数。
type InputGuard func(text string) (string, error)

type inputGuardKey struct{}

// WithInputGuard は、このコンテキストで行う外部 API 呼び出しのユーザー入力（ユーザープロンプト・埋め込み入力）に、
// 送る前に g をかけるようにする。システムプロンプトは固定文なので対象外。
//
// 学生の文章を扱う入口（チャットなど）で一度付ければ、下流の評価・判定・質問生成・埋め込みがすべて覆われる。
// ガードが無いコンテキスト（企業調査など、学生の文章を含まない呼び出し）は、何も変わらない。
func WithInputGuard(ctx context.Context, g InputGuard) context.Context {
	return context.WithValue(ctx, inputGuardKey{}, g)
}

// WithoutInputGuard は、ctx に付いたガードを外す。学生の文章を含まない呼び出し（企業名だけを送る読み仮名の問い合わせなど）用。
// 企業名を伏字にすると、学生の氏名の一部が企業名に含まれたときに結果が崩れ、共有のキャッシュにも残る。
func WithoutInputGuard(ctx context.Context) context.Context {
	return WithInputGuard(ctx, nil)
}

// studentDerivedFeatures は、学生の文章をユーザー入力として送る機能名（usagectx.WithFeature）の一覧。
// この機能名の呼び出しにガードが付いていなければ、付け忘れとみなして送らない（fail-closed。SOCAIAGENT-697）。
// 学生の文章を含まない呼び出しがこの機能名を使うときは、WithoutInputGuard で明示的に外す。
//
// 入れていないもの:
//   - ES 添削・リライト・履歴書: ガードではなく、送る前に本文を明示的に伏字にする方式（piimask.ForLLM）で守っている。
//   - 面接の STT・TTS: 音声、または面接官（AI）の発話で、ここ（テキストのユーザー入力）を通らない。
//   - マッチング理由: 送るのはスコアと企業情報で、学生の文章ではない。
//
// 学生の文章を送る機能名を新しく足したら、ここにも足す。
var studentDerivedFeatures = map[string]struct{}{
	usagectx.FeatureChatAnswerEval:    {},
	usagectx.FeatureChatAnswerCheck:   {},
	usagectx.FeatureChatSummary:       {},
	usagectx.FeatureChatQuestion:      {},
	usagectx.FeatureEmbedding:         {},
	usagectx.FeatureAnalysisScoring:   {},
	usagectx.FeatureInterviewTurn:     {},
	usagectx.FeatureInterviewReport:   {},
	usagectx.FeatureInterviewRealtime: {},
	usagectx.FeatureDiagnosisQuality:  {},
	usagectx.FeatureGitHubSummary:     {},
}

// guardInput は、コンテキストにガードがあれば text にかける。ガードが失敗したら送らない。
// ガードが無く、学生由来の機能名が付いているときも送らない（WithoutInputGuard で明示的に外したときは送る）。
func guardInput(ctx context.Context, text string) (string, error) {
	g, set := ctx.Value(inputGuardKey{}).(InputGuard)
	if g == nil {
		if !set {
			if _, ok := studentDerivedFeatures[usagectx.Feature(ctx)]; ok {
				return "", fmt.Errorf("%w: 学生由来の呼び出し（%s）に入力ガードが付いていません", ErrInputGuardFailed, usagectx.Feature(ctx))
			}
		}
		return text, nil
	}
	out, err := g(text)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInputGuardFailed, err)
	}
	return out, nil
}
