package piimask

import (
	"strings"
	"testing"
)

// 履歴書で実際に書かれる、個人情報の書き方のばらつき。どれも伏字になること。すべて合成データ。
//
// 伏字にしないと決めているもの（取りこぼしではなく、設計どおり）:
//   - 本文中のふりがな・カタカナの氏名（漢字で登録した氏名から、読みは導けない）。
//     履歴書の「ふりがな」欄とその値のブロックは、行ごと除く（resume_llm_input_test.go）。
//   - 番地の無い町名（「東京都港区六本木」の「六本木」）、年齢だけ（「22歳の学生です」）、本人の学校ではない学校名。
func TestMask_RealisticVariants(t *testing.T) {
	m := mustMasker(t, Subject{Name: "山田 太郎", Email: "taro.yamada@example.com", SchoolName: "架空情報専門学校"})
	tests := []struct{ in, leak string }{
		// 氏名（登録名の揺れ・ローマ字）
		{"山田　太郎です", "山田"}, {"山田です。", "山田"}, {"太郎と呼ばれています", "太郎"}, {"山 田 太 郎", "山"}, {"山田様", "山田"},
		{"Yamada Taro です", "Yamada"}, {"TARO YAMADA", "TARO"}, {"Taro Yamada", "Taro"},
		// メール
		{"ＴＡＲＯ＠example.com", "example"}, {"taro+job@example.com", "example"}, {"TARO.YAMADA@EXAMPLE.COM", "EXAMPLE"}, {"taro (at) example.com", "example"},
		// 電話
		{"09012345678", "0901234"}, {"090 1234 5678", "1234"}, {"０９０－１２３４－５６７８", "１２３４"}, {"+81 90-1234-5678", "1234"},
		{"03(1234)5678", "1234"}, {"(090)1234-5678", "1234"}, {"TEL:090.1234.5678", "1234"}, {"0120-123-456", "123-456"}, {"携帯 090 - 1234 - 5678", "1234"},
		// 住所（都道府県の省略を含む）
		{"東京都港区六本木1丁目2番3号 〇〇マンション101", "六本木"}, {"港区六本木1-2-3", "六本木"}, {"大阪市北区梅田1-1", "梅田"},
		{"神奈川県横浜市中区山下町1", "山下"}, {"千葉県船橋市本町2-3-4", "本町"}, {"札幌市中央区北1条西2丁目", "北1条"},
		// 生年月日
		{"生年月日 1999-04-01", "1999"}, {"1999/04/01生", "1999"}, {"1999.4.1 生まれ", "1999"}, {"H11.4.1生", "H11"}, {"平成十一年四月一日生", "十一"},
		{"04/01/1999", "1999"}, {"1999年4月1日（22歳）", "1999"}, {"昭和・平成 11年4月1日生", "11年"}, {"生年月日：平成11年4月1日", "平成11"}, {"2001年4月1日生まれの22歳", "2001"},
		// 郵便番号・URL・SNS
		{"〒1060032", "1060032"}, {"106-0032", "106-0032"}, {"github.com/taro-yamada", "taro-yamada"}, {"twitter.com/taro_dev", "taro_dev"},
		{"linkedin.com/in/taro-yamada", "taro-yamada"}, {"qiita.com/taro", "taro"}, {"note.com/taro_note", "taro_note"}, {"https://example.com/~taro", "taro"},
		{"X: @taro_dev", "taro_dev"}, {"LINE ID: taro123", "taro123"}, {"Instagram: taro_dev", "taro_dev"},
		// 本人に結びつく番号
		{"学籍番号: 20A1234", "20A1234"}, {"学籍番号 20230001", "20230001"}, {"マイナンバー 1234 5678 9012", "5678"}, {"受験番号 A12345", "A12345"},
		// 学校名（登録情報）
		{"架空情報専門学校 卒業", "架空"},
	}
	for _, tt := range tests {
		if got := m.Mask(tt.in); strings.Contains(got, tt.leak) {
			t.Errorf("伏字になっていない: %q → %q（%q が残る）", tt.in, got, tt.leak)
		}
	}
}

// 普通の履歴書・ESの文は、伏字で壊れない。規則を足すたびに、ここで誤検出を確かめる。
func TestMask_DoesNotBreakOrdinaryProse(t *testing.T) {
	m := mustMasker(t, Subject{Name: "山田 太郎", Email: "taro.yamada@example.com", SchoolName: "架空情報専門学校"})
	keep := []string{
		"渋谷区の会社で3年間、Webアプリの開発を担当しました。", "港区在住の学生ともチームを組みました。", "2024年4月1日に入社し、新人研修を受けました。",
		"2023年3月31日に卒業しました。", "令和7年3月 卒業見込み", "売上を前年比120%に伸ばしました。", "10名のチームでリーダーを務めました。",
		"使用ツール: GitHub, Docker, Slack, Notion", "GitHub: Docker と Kubernetes を使用", "Xの運用を担当し、フォロワーを増やしました。",
		"LINEアプリの開発に参加しました。", "Instagram: 投稿の企画を担当", "Slack: 連絡の効率化", "taxiアプリの配車ロジックを改善しました。",
		"footnote.comのような記事サイトの改善をしました。", "3/4の学生が参加しました。", "R&Dの経験があります。", "TOEIC 780点を取得しました。",
		"第3回大会で優勝しました。", "東京都の大学で学びました。", "大阪府出身で、関西弁が話せます。", "1-2-3の順で作業しました。",
		"アルバイトで時給1,200円から1,350円に上がりました。", "令和6年度の目標は達成率120%でした。", "Webサイト(https化)の対応をしました。",
		"株式会社サンプルのインターンで3か月、Webの開発を担当しました。", "チームで10名のプロジェクトを進めました。",
	}
	for _, s := range keep {
		if got := m.Mask(s); got != s {
			t.Errorf("普通の文が変わった:\n  in : %q\n  out: %q", s, got)
		}
	}
}

func TestIsBareContactLabel(t *testing.T) {
	for _, s := range []string{"氏名", "ふりがな", "生年月日", "住所", "電話", "メール", "氏名:", "年齢"} {
		if !IsBareContactLabel(s) {
			t.Errorf("見出しだけの行のはず: %q", s)
		}
	}
	for _, s := range []string{"氏名 山田太郎", "志望動機", "電話応対の経験", "", "自己PR"} {
		if IsBareContactLabel(s) {
			t.Errorf("見出しだけの行ではないはず: %q", s)
		}
	}
}

// 大文字入りの URL・全角の「＠」のアカウント名も伏字になること（レビューでの指摘）。合成データ。
func TestMask_CaseAndFullWidthVariants(t *testing.T) {
	m := mustMasker(t, Subject{})
	for _, tt := range []struct{ in, leak string }{
		{"Github.com/hanako-dev", "hanako-dev"}, {"HTTP://EXAMPLE.COM/me", "EXAMPLE.COM"}, {"＠hanako_dev", "hanako_dev"},
	} {
		if got := m.Mask(tt.in); strings.Contains(got, tt.leak) {
			t.Errorf("%q が残った: %q", tt.in, got)
		}
	}
}

// 技術スキルや年月の普通の文を、アカウント名・生年月日と取り違えて壊さないこと（レビューでの指摘）。
func TestMask_KeepsSkillAndDateProse(t *testing.T) {
	m := mustMasker(t, Subject{})
	for _, in := range []string{
		"MAX: 100 人規模", "UI/UX: 3年の経験", "Mac OS X: 10.15", "HEADLINE: 2024",
		"2024/4/1 生成AIを導入", "2024/4/1 生産管理を担当", "12/25/2023 に開催",
	} {
		if got := m.Mask(in); got != in {
			t.Errorf("普通の文が変わった: %q → %q", in, got)
		}
	}
}
