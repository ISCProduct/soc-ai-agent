package services

import "testing"

func TestIsDeveloperOnlyReleaseNote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		title string
		body  string
		want  bool
	}{
		{name: "学生向け新機能は残す", title: "feat: 更新情報ページを追加", body: "学生が変更点を確認できる", want: false},
		{name: "空の入力は残す", title: "", body: "", want: false},
		{name: "docsプレフィックスは開発者専用ではない", title: "docs: ユーザー向けヘルプを更新", body: "", want: false},
		{name: "ciプレフィックスは除外", title: "ci: Dockerビルドキャッシュを変更", body: "", want: true},
		{name: "CIプレフィックス大文字は除外", title: "CI: workflow timeout", body: "", want: true},
		{name: "choreプレフィックスは除外", title: "chore: 依存関係を更新", body: "", want: true},
		{name: "opsプレフィックスは除外", title: "ops: staging ASGを調整", body: "", want: true},
		{name: "terraformは除外", title: "fix: 本番ネットワーク", body: "Terraformのルートテーブルを修正", want: true},
		{name: "GitHub Actionsは除外", title: "fix: テスト実行", body: "GitHub Actions のジョブ分割", want: true},
		{name: "workflowsパスは除外", title: "fix: 通知", body: ".github/workflows/test.yml を更新", want: true},
		{name: "CodeRabbit指摘は除外", title: "fix: CodeRabbit指摘への対応", body: "", want: true},
		{name: "Fargateは除外", title: "feat: 本番起動", body: "ECS Fargate の desired_count", want: true},
		{name: "docker composeは除外", title: "fix: ローカル起動", body: "docker compose のヘルスチェック", want: true},
		{name: "infraパスは除外", title: "fix: モジュール", body: "infra/terraform/modules/ecr", want: true},
		{name: "レビュー指摘は除外", title: "fix: レビュー指摘を反映", body: "", want: true},
		{name: "インフラ構成は除外", title: "インフラ構成を更新", body: "", want: true},
		{name: "デプロイパイプラインは除外", title: "改善", body: "デプロイパイプラインの安定化", want: true},
		{name: "CI/CDは除外", title: "速度改善", body: "CI/CD の並列化", want: true},
		{name: "保存済みのやさしい要約でもTerraformなら除外", title: "構成を更新しました", body: "Terraform構成を直しました。", want: true},
		// Release 傘PRは本文に Fargate/Terraform 定型文があっても LLM に任せる
		{name: "Release傘PRのFargate定型文は除外しない", title: "Release to production: 面接UX修正", body: "マージすると本番（ECS on Fargate）へ自動デプロイされます。", want: false},
		{name: "Release傘PRのterraform言及は除外しない", title: "Release: 2026-09-07 レート制限回避ほか", body: "infra/terraform の修正を含む", want: false},

		// #1289: Release 傘PRでも「タイトルが運用作業そのもの」なら除外する。
		// 本番の更新情報に Discord からの環境起動/停止などが表示されていた。
		// 実際に本番へ出た PR タイトルをそのまま使う。
		{name: "実例: Discordから本番環境を起動/停止するは除外", title: "Release: 2026-09-09 本番反映（Discordから本番環境を起動/停止する）", body: "", want: true},
		{name: "実例: Lambda移設とstaging運用は除外", title: "release: Discord受け口のLambda移設と観測性・staging運用の修正を本番ブランチへ反映する", body: "", want: true},
		{name: "実例: staging復旧まわりは除外", title: "release: staging復旧まわりの修正4件を本番ブランチへ反映する", body: "", want: true},
		{name: "実例: 基盤バージョンアップとデプロイ基盤は除外", title: "release: 基盤バージョンアップ(#1277)とデプロイ基盤の修正を本番へ反映する", body: "", want: true},
		{name: "実例: 本番RAGインフラのコード化は除外", title: "Release: 2026-08-31 本番RAGインフラ(rag-review/chroma)のコード化", body: "", want: false},

		// ユーザー向けリリースまで落とさないこと（フィルタの過剰適用を防ぐ回帰）
		{name: "実例: 音声認識改善・教員向け機能は残す", title: "Release: 2026-09-10 本番反映（AI面接の音声認識改善・教員向け機能・公開API漏洩修正）", body: "", want: false},
		{name: "実例: 教員向け分析・企業パスワードリセットは残す", title: "Release: 2026-09-08 本番反映（教員向け生徒傾向分析・企業パスワードリセット・未審査企業の露出修正）", body: "", want: false},
		{name: "実例: スカウト機能は残す", title: "Release to production: 2026-09-05 スカウト機能、OpenAIリトライ不具合修正、CI検証範囲の拡大", body: "", want: false},
		{name: "実例: 面接深掘り継続力は残す", title: "Release to production: 2026-09-05 面接深掘り継続力・OAuth文字化け修正・FT蒸留排除・Backlog同期", body: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isDeveloperOnlyReleaseNote(tt.title, tt.body)
			if got != tt.want {
				t.Fatalf("isDeveloperOnlyReleaseNote(%q, %q)=%v, want %v", tt.title, tt.body, got, tt.want)
			}
		})
	}
}
