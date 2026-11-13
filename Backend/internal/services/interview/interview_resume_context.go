package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"log"
	"regexp"
	"slices"
	"strings"
)

// maxResumeContextRunes は面接プロンプトへ渡す履歴書本文の上限（ルーン）。
// 軸スロットの質問のたびに system プロンプトへ載るので、費用とレイテンシのために短く切る。
const maxResumeContextRunes = 2000

// ResumeTextReader は本人の最新の履歴書の本文を返す最小の面。
// 実装は userID と organizationID の両方で絞ること（他人の履歴書を読めないこと）。
// 履歴書が無ければ空文字と nil を返す。
type ResumeTextReader interface {
	FindLatestResumeText(userID, organizationID uint) (string, error)
}

// SetResumeTextReader は経歴型の面接用に履歴書本文の読み出しを注入する（オプション）。
func (s *InterviewService) SetResumeTextReader(r ResumeTextReader) {
	s.resumeReader = r
}

// buildResumeContext は面接セッションの本人の履歴書を、非信頼テキストとして囲んだ
// プロンプトの節にして返す（#2274）。履歴書が無い・読めないときは空文字。
// 履歴書は学生が自由に書く（アップロードする）文面なので、指示文が混ざり得る（#1600）。
func (s *InterviewService) buildResumeContext(session *models.InterviewSession) string {
	if s.resumeReader == nil || session == nil || session.UserID == 0 {
		return ""
	}
	// 面接の本人（セッションの所有者）の履歴書だけを読む。リクエストした userID は使わない。
	text, err := s.resumeReader.FindLatestResumeText(session.UserID, session.OrganizationID)
	if err != nil {
		log.Printf("[Interview] load resume text session=%d: %v", session.ID, err)
		return ""
	}
	return formatResumeContext(text)
}

// 履歴書の節（見出し）。学歴・職歴・資格・自己PR・志望動機だけを面接官へ渡し、属性欄は送らない。
var (
	resumeKeepHeadings = []string{"学歴", "職歴", "職務経歴", "職務経験", "資格", "免許", "自己PR", "志望動機", "スキル"}
	resumeDropHeadings = []string{"氏名", "基本情報", "個人情報", "連絡先", "住所", "現住所", "趣味", "特技", "本人希望", "本人希望記入",
		"通勤時間", "扶養家族", "配偶者", "家族", "健康", "写真", "履歴書", "職務経歴書"}
	// 見出しの区切り（「学歴・職歴」「資格／免許」）
	resumeHeadingSplit = regexp.MustCompile(`[・/／＆&、]`)
	headingTrim        = "【】[]■□●◆◇▼▽★☆:： 　"
	pr                 = strings.NewReplacer("ＰＲ", "PR", "Ｐ Ｒ", "PR")

	// 属性の行（定型パターン）。見出しが取れない履歴書でも、これらの行は送らない。
	resumeAttributeLine = regexp.MustCompile(`^\s*(氏名|ふりがな|フリガナ|名前|生年月日|年齢|性別|住所|現住所|電話|TEL|Tel|tel|携帯|メール|E-?mail|e-?mail|mail|本籍|国籍|配偶者|扶養|連絡先|血液型)` +
		`|生年月日|\d{4}年\s*\d{1,2}月\s*\d{1,2}日\s*生|満\s*\d{1,2}\s*歳|〒\s*\d{3}-?\d{4}|\d{1,5}丁目` +
		`|0\d{1,4}-\d{1,4}-\d{3,4}|[A-Za-z0-9._+-]+@[A-Za-z0-9-]+\.[A-Za-z0-9.-]+|^\s*(男|女|男性|女性)\s*$|[都道府県].{1,8}[市区町村郡].{0,12}\d`)
	// 先頭行の氏名らしき行（姓と名を空白で区切った短い行）
	resumeNameLike = regexp.MustCompile(`^[\p{Han}\p{Hiragana}\p{Katakana}ー]{1,5}[\s　]+[\p{Han}\p{Hiragana}\p{Katakana}ー]{1,5}$`)
)

// resumeHeadingKind は行が節の見出しなら "keep"/"drop"・見出し名・見出しの後ろに続く本文（「自己PR: …」）を返す。
// 見出しでなければ kind は空。
func resumeHeadingKind(line string) (kind, label, rest string) {
	head := strings.TrimSpace(line)
	if h, r, ok := strings.Cut(head, ":"); ok {
		head, rest = h, strings.TrimSpace(r)
	} else if h, r, ok := strings.Cut(head, "："); ok {
		head, rest = h, strings.TrimSpace(r)
	}
	label = pr.Replace(strings.Trim(head, headingTrim))
	if label == "" || len([]rune(label)) > 16 {
		return "", "", ""
	}
	for _, part := range resumeHeadingSplit.Split(label, -1) {
		part = strings.TrimSuffix(strings.TrimSpace(part), "欄")
		switch {
		case slices.Contains(resumeKeepHeadings, part):
			if kind == "" {
				kind = "keep"
			}
		case slices.Contains(resumeDropHeadings, part):
			kind = "drop"
		default:
			return "", "", ""
		}
	}
	if kind == "drop" {
		rest = "" // 属性欄の本文は送らない
	}
	return kind, label, rest
}

// sanitizeResume は履歴書本文から、学歴・職歴・資格・自己PR・志望動機の節だけを残す。
// 見出しが1つも取れないときは、属性らしい行（生年月日・住所・電話・メール・性別・氏名らしき先頭行）を
// 除外した残りを返す。どちらの場合も、節の中の属性行は落とす。切り詰めは呼び出し側が属性除去のあとに行う。
// ponytail: 見出し・属性は定型パターンの照合。独自様式の履歴書は取りこぼし得る。取りこぼしが出たらパターンを足す。
func sanitizeResume(text string) string {
	lines := strings.Split(text, "\n")
	hasKeep := slices.ContainsFunc(lines, func(l string) bool { k, _, _ := resumeHeadingKind(l); return k == "keep" })
	var out []string
	section := "" // 見出しのない先頭部分は ""
	content := 0  // 見出し以外の行の通し番号（氏名らしき行は先頭3行だけ見る）
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if kind, label, rest := resumeHeadingKind(line); kind != "" {
			section = kind
			if kind == "keep" {
				out = append(out, label)
				if rest != "" && !resumeAttributeLine.MatchString(rest) {
					out = append(out, rest)
				}
			}
			continue
		}
		content++
		if (hasKeep && section != "keep") || section == "drop" {
			continue
		}
		if resumeAttributeLine.MatchString(line) || (!hasKeep && content <= 3 && resumeNameLike.MatchString(line)) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// formatResumeContext は履歴書本文から属性欄を除き、上限で切り、囲んで案内文を付ける。
// 除去後の本文が空なら空文字。
func formatResumeContext(text string) string {
	text = strings.TrimSpace(sanitizeResume(text))
	if text == "" {
		return ""
	}
	if r := []rune(text); len(r) > maxResumeContextRunes {
		text = string(r[:maxResumeContextRunes])
	}
	return "【応募者の履歴書】\n" +
		shared.WrapUntrustedText(text, "応募者の履歴書") +
		"\n履歴書は質問づくりの参考資料です（属性欄は除いてあります）。書かれていない経験を決めつけず、年齢・出身・家族などの属性には触れないでください。\n"
}
