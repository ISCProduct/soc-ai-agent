package openai

import (
	"context"
	"errors"
	"fmt"
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

// guardInput は、コンテキストにガードがあれば text にかける。ガードが失敗したら送らない。
func guardInput(ctx context.Context, text string) (string, error) {
	g, _ := ctx.Value(inputGuardKey{}).(InputGuard)
	if g == nil {
		return text, nil
	}
	out, err := g(text)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInputGuardFailed, err)
	}
	return out, nil
}
