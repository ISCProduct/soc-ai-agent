package resume

import (
	"errors"
	"fmt"
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/piimask"
)

// llmBlocks は、LLM と RAG へ渡してよい（伏字済みの）ブロック。
//
// 学生の履歴書の本文には、氏名・住所・電話・メール・生年月日が入る。外部へ送る関数
// （buildResumeText など）は、この型だけを受け取る。maskBlocksForLLM / maskBlocksWith を通さずに
// 元のブロック（[]models.ResumeTextBlock）を渡そうとすると、コンパイルが通らない。
// DB へ保存する本文と注釈PDFの位置は元のブロックのままで、伏字にするのは外へ送る写しだけ。
//
// 名前つきのスライス型（type llmBlocks []T）にしないのは、Go が型リテラルの []T を暗黙に代入できてしまい、
// 元のブロックがそのまま通るため。構造体で包むと、暗黙の代入ができない。
type llmBlocks struct {
	items []models.ResumeTextBlock
}

// list は伏字済みのブロックを返す（位置・番号の照合に使う）。
func (b llmBlocks) list() []models.ResumeTextBlock { return b.items }

// errMaskUnavailable は、伏字の準備ができなかったときのエラー。このとき本文を外へ送ってはならない。
var errMaskUnavailable = errors.New("個人情報を安全に処理できなかったため、レビューできませんでした。しばらくしてからもう一度お試しください")

// SetSubjectReader は、学生の登録情報（氏名・メール・学校名）を読む処理を注入する。
// 伏字の辞書に使う。注入しなくても、規則（メール・電話・住所など）での伏字は必ず行う。
func (s *ResumeService) SetSubjectReader(r piimask.SubjectReader) {
	s.subjects = r
}

// maskBlocksForLLM は、userID の学生の登録情報で辞書を作って、ブロックを伏字にする。
// 登録情報を読めなかったときはエラーを返す。呼び出し側は、本文を外へ送ってはならない（失敗したら送らない）。
func (s *ResumeService) maskBlocksForLLM(userID uint, blocks []models.ResumeTextBlock) (llmBlocks, error) {
	var subj piimask.Subject
	if s.subjects != nil {
		got, err := s.subjects.SubjectFor(userID)
		if err != nil {
			return llmBlocks{}, fmt.Errorf("%w: %v", errMaskUnavailable, err)
		}
		subj = got
	}
	m, err := piimask.ForLLM(subj)
	if err != nil {
		return llmBlocks{}, fmt.Errorf("%w: %v", errMaskUnavailable, err)
	}
	return maskBlocksWith(m, blocks), nil
}

// maskBlocksWith は、連絡先の欄の行を除き、残りの本文を伏字にしたブロックを返す。
// ページ番号・ブロックの番号・位置（BBox）・認識スコアは変えない。指摘は番号で元のブロックに対応づく。
func maskBlocksWith(m *piimask.Masker, blocks []models.ResumeTextBlock) llmBlocks {
	out := make([]models.ResumeTextBlock, 0, len(blocks))
	for _, b := range blocks {
		if piimask.IsContactFieldLine(b.Text) {
			continue
		}
		b.Text = m.Mask(strings.TrimSpace(b.Text))
		out = append(out, b)
	}
	return llmBlocks{items: out}
}
