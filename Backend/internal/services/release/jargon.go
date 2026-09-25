package release

// 更新情報から専門用語を締め出す（#1290 の再発防止）。
//
// 読み手はパソコンやITに詳しくない学生・教員。プロンプトで「専門用語を使うな」と
// 指示しても従わないことがあるため、出力側でも検査する。
//
// ここで弾かれた要約は保存しない。専門用語まじりの文章を出すくらいなら、
// その回の更新情報を出さないほうがよい。

import "strings"

// jargonNeedles は更新情報に出してはいけない語。
//
// 「その語が入っていたら利用者に伝わらない」ものだけを入れる。
// 一般語（例: 「画面」「保存」）を入れると、正しい要約まで落ちる。
var jargonNeedles = []string{
	// 技術一般
	"api", "ui", "ux", "バッチ", "キャッシュ", "セッション", "デプロイ",
	"パフォーマンス", "アルゴリズム", "データベース", "サーバー", "エンドポイント",
	"マイグレーション", "リファクタ", "レスポンス", "リクエスト", "クエリ",
	"トークン", "webhook", "ウェブフック", "スキーマ", "ロジック",
	// 基盤・運用
	"aws", "ecs", "rds", "lambda", "terraform", "docker", "ci/cd",
	"cloudwatch", "fargate", "インフラ", "ジョブ", "cron", "スクリプト",
	// 開発フロー
	"リポジトリ", "マージ", "コミット", "ブランチ", "プルリク",
}

// containsJargon は要約に専門用語が残っているかを返す。
func containsJargon(texts ...string) (string, bool) {
	lower := strings.ToLower(strings.Join(texts, "\n"))
	for _, needle := range jargonNeedles {
		if strings.Contains(lower, needle) {
			return needle, true
		}
	}
	return "", false
}
