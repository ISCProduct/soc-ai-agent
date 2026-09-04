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
