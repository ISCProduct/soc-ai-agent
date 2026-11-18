package resume

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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

// maxValueAfterLabelRunes は、見出しだけの行の次のブロックを「値」とみなす長さの上限。
// 長ければ本文なので残す（伏字にはかける）。
const maxValueAfterLabelRunes = 40

// maxContinuationBlocks は、住所・電話・メールの欄の行のあとに続けて除く、続きのブロックの数の上限。
const maxContinuationBlocks = 2

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
	// OCR は行ごとにブロックを分けるので、1つの値が複数のブロックにまたがることがある。
	//   - 見出しだけの行（「氏名」）の次のブロックは値。辞書にない書き方（ふりがな・ローマ字）も残さず除く。
	//   - 住所・電話・メールの欄の行の次に、番地・建物名・電話やメールの続きが来たら、それも除く（最大2ブロック）。
	//     見出しだけの行（「住所」）は、値のブロックに加えて、続きも最大2ブロック除く（合計で最大3ブロック）。
	//   - 「…@」で終わるブロックの次の先頭は、メールのドメインの続き。
	afterBareLabel := false
	continuations := 0
	prevEndsWithAt := false
	for _, b := range blocks {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			// 空のブロックは、見出しや続きの状態を変えない（OCR が空白だけのブロックを挟んでも、値を残さない）
			b.Text = ""
			out = append(out, b)
			continue
		}
		if piimask.IsContactFieldLine(text) {
			afterBareLabel = piimask.IsBareContactLabel(text)
			continuations = 0
			if piimask.IsContinuationLabelLine(text) {
				continuations = maxContinuationBlocks
			}
			prevEndsWithAt = false
			continue
		}
		if afterBareLabel {
			afterBareLabel = false
			if utf8.RuneCountInString(text) <= maxValueAfterLabelRunes {
				continue
			}
		}
		if continuations > 0 {
			if piimask.IsContactContinuation(text) {
				continuations--
				continue
			}
			continuations = 0
		}
		if prevEndsWithAt {
			text = piimask.MaskLeadingDomain(text)
		}
		b.Text = m.Mask(text)
		prevEndsWithAt = piimask.EndsWithAt(b.Text)
		out = append(out, b)
	}
	return llmBlocks{items: out}
}
