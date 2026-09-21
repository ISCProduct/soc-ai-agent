package release

// 更新情報から専門用語を締め出すテスト（#1290）。
// 実行: cd Backend && go test ./internal/services/release/ -run Jargon -v
//
// 読み手はパソコンやITに詳しくない学生・教員。プロンプトで指示しても
// 従わないことがあるため、出力側でも検査する。

import "testing"

func TestContainsJargon_専門用語を検出する(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		summary string
		want    bool
	}{
		// 通すべきもの。利用者から見て何が変わったかが書かれている。
		{"やさしい要約", "面接の質問が改善", "面接練習で聞かれる質問が、あなたの回答に合わせて変わるようになりました。", false},
		{"読み込みの速さ", "表示が速くなりました", "企業一覧の表示を待つ時間が短くなりました。", false},
		{"保存の不具合", "入力内容が消える問題を修正", "履歴書を書いている途中で入力した内容が消えてしまう問題を直しました。", false},

		// 落とすべきもの。専門用語が残っている。
		{"APIという語", "機能を追加", "APIを追加しました。", true},
		{"パフォーマンス", "改善しました", "パフォーマンスを改善しました。", true},
		{"UI", "UIを改善", "画面を見やすくしました。", true},
		{"デプロイ", "更新", "デプロイの手順を変えました。", true},
		{"キャッシュ", "高速化", "キャッシュを導入しました。", true},
		{"タイトル側に専門用語", "バッチ処理の改善", "まとめて処理できるようになりました。", true},
		{"大文字のAWS", "構成変更", "AWSの設定を変えました。", true},
		{"アルゴリズム", "おすすめ改善", "マッチングのアルゴリズムを見直しました。", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needle, got := containsJargon(tt.title, tt.summary)
			if got != tt.want {
				t.Errorf("got %v (needle=%q), want %v\n  title=%q\n  summary=%q",
					got, needle, tt.want, tt.title, tt.summary)
			}
		})
	}
}

func TestContainsJargon_一般語で誤検出しない(t *testing.T) {
	// 一般語を needle に入れると、正しい要約まで落ちる。
	// 利用者向けの自然な文章が通ることを確認する。
	ok := []string{
		"企業の検索結果が見やすくなりました。",
		"面接の練習結果を保存できるようになりました。",
		"自己PRの添削が受けられるようになりました。",
		"おすすめの企業があなたの希望に近づきました。",
		"先生が生徒の進み具合を確認できるようになりました。",
	}
	for _, s := range ok {
		if needle, found := containsJargon("", s); found {
			t.Errorf("誤検出: %q が %q で落ちた", s, needle)
		}
	}
}
