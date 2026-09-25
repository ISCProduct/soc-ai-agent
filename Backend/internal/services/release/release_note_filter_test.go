package release

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
		// release 傘PRは一律で除外する（#1290 の再発防止）。
		//
		// main へ直接マージされるのは傘PRだけで、その本文は運用担当者向けに書かれる。
		// 以前はタイトルのニードル判定に頼っていたが、傘PRのタイトルは中身の要約に
		// なっており運用語が入らないことがある。実際に
		// 「SRE整備（通知・レート制限・可観測性）とAI利用量計測ほか34件」が通過し、
		// 本文の「起動ジョブの失敗通知が初めて有効になる」が学生向けに表示された。
		//
		// 更新情報は中身の個別PRから作る（collect_whats_new_sources.py）。
		{name: "傘PR: 運用語を含まないタイトルでも除外", title: "release: 本番反映 — SRE整備（通知・レート制限・可観測性）とAI利用量計測ほか34件", body: "", want: true},
		{name: "傘PR: ユーザー向けの内容でも除外", title: "Release: 2026-09-10 本番反映（AI面接の音声認識改善・教員向け機能・公開API漏洩修正）", body: "", want: true},
		{name: "傘PR: 大文字小文字を問わず除外", title: "Release to production: 面接UX修正", body: "", want: true},
		{name: "傘PR: 企業ポータルほか21件も除外", title: "release: 本番反映 — 企業ポータル・ゲスト診断の引き継ぎ・掲載承認前求人の露出修正ほか21件", body: "", want: true},

		// 個別PRは主題がタイトルに出るので判定できる。ユーザー向けは残す。
		{name: "個別PR: 機能追加は残す", title: "feat(interview): 面接の深掘り質問を改善する", body: "", want: false},
		{name: "個別PR: 不具合修正は残す", title: "fix(resume): 履歴書レビューの結果が保存されない問題を直す", body: "", want: false},
		{name: "個別PR: 運用は除外", title: "ops: 起動/停止ジョブの失敗をDiscordへ通知する", body: "", want: true},
		// スコープ側が運用のこともある。型(fix)だけ見ると通ってしまう。
		{name: "個別PR: fix(ops)は除外", title: "fix(ops): 本番の起動判定をAWS側のスケジューラで発火させる", body: "", want: true},
		{name: "個別PR: chore(deps)は除外", title: "chore(deps): bump golang.org/x/net", body: "", want: true},
		{name: "個別PR: fix(infra)は除外", title: "fix(infra): 設定を直す", body: "", want: true},
		// 機能スコープの fix は残す。
		{name: "個別PR: fix(diagnosis)は残す", title: "fix(diagnosis): 出題した軸を採点まで運ぶ", body: "", want: false},
		{name: "個別PR: fix(resume)は残す", title: "fix(resume): レビュー結果を保存する", body: "", want: false},
		{name: "個別PR: リファクタは除外", title: "refactor(backend): services 直下の取り残しをサブパッケージへ移す", body: "", want: true},
		{name: "個別PR: ビルドは除外", title: "build(deps): bump golang.org/x/net", body: "", want: true},
		{name: "個別PR: テストは除外", title: "test: 権限境界の検査を足す", body: "", want: true},
		{name: "個別PR: 性能改善は除外", title: "perf(matching): 計算を速くする", body: "", want: true},
		// docs: は一律除外しない。利用者向けヘルプの更新が混ざるため本文で判定する。
		{name: "個別PR: 開発者向けドキュメントは本文で除外", title: "docs: migrations の手順を追記する", body: "デプロイ手順を追記", want: true},
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
