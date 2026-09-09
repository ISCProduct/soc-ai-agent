package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// WorkflowDispatcher は本番の起動状態を反映する GitHub Actions ワークフローを起動する。
//
// 反映処理そのもの（RDS起動待ち、chroma→rag-review→backend→frontend の順序、
// オートスケーリング下限の同期）は prod-uptime-scheduler.yml が持っている。
// ここで同じ手順をGoに書き直すと二重管理になり、片方だけ直して本番が中途半端な状態で
// 起動する事故につながるため、既存のワークフローをそのまま呼ぶ。
//
// トークン未設定でも機能は壊さない。その場合は次の毎時実行で反映される。
type WorkflowDispatcher struct {
	token    string
	repo     string
	workflow string
	ref      string
	baseURL  string
	client   *http.Client
}

const (
	defaultUptimeWorkflowFile = "prod-uptime-scheduler.yml"
	defaultGitHubAPIBaseURL   = "https://api.github.com"
)

func NewWorkflowDispatcherFromEnv() *WorkflowDispatcher {
	d := &WorkflowDispatcher{
		token:    os.Getenv("GITHUB_DISPATCH_TOKEN"),
		repo:     os.Getenv("GITHUB_DISPATCH_REPO"),
		workflow: os.Getenv("PROD_UPTIME_WORKFLOW_FILE"),
		ref:      os.Getenv("GITHUB_DISPATCH_REF"),
		baseURL:  defaultGitHubAPIBaseURL,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
	if d.workflow == "" {
		d.workflow = defaultUptimeWorkflowFile
	}
	if d.ref == "" {
		// workflow_dispatch はデフォルトブランチ上のワークフロー定義で動く
		d.ref = "main"
	}
	return d
}

// Enabled は即時反映を実行できる設定が揃っているかを返す。
func (d *WorkflowDispatcher) Enabled() bool {
	return d != nil && d.token != "" && d.repo != ""
}

// Dispatch はワークフローを起動する。設定が無ければ (false, nil) を返す（エラーではない）。
func (d *WorkflowDispatcher) Dispatch(ctx context.Context) (bool, error) {
	return d.DispatchWorkflow(ctx, d.workflow)
}

// DispatchWorkflow はワークフローを指定して起動する（#1249）。
//
// 本番とステージングで別のワークフローを叩くため、ファイル名を引数に取る。
// 空文字なら既定（PROD_UPTIME_WORKFLOW_FILE）を使う。
func (d *WorkflowDispatcher) DispatchWorkflow(ctx context.Context, workflow string) (bool, error) {
	if d == nil || !d.Enabled() {
		return false, nil
	}
	if strings.TrimSpace(workflow) == "" {
		workflow = d.workflow
	}

	body, err := json.Marshal(map[string]string{"ref": d.ref})
	if err != nil {
		return false, fmt.Errorf("リクエストの組み立てに失敗しました: %w", err)
	}

	base := d.baseURL
	if base == "" {
		base = defaultGitHubAPIBaseURL
	}
	url := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/dispatches", base, d.repo, workflow)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("リクエストの作成に失敗しました: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("ワークフローの起動に失敗しました: %w", err)
	}
	defer resp.Body.Close()

	// 成功時は 204 No Content
	if resp.StatusCode != http.StatusNoContent {
		return false, fmt.Errorf("ワークフローの起動に失敗しました: HTTP %d", resp.StatusCode)
	}
	return true, nil
}
