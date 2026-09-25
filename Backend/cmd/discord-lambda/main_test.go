package main

// Lambda の入口のルーティングテスト（#1388）。
// 実行: cd Backend && go test ./cmd/discord-lambda/ -v
//
// 同じ Lambda が ALB（Discord Interaction）と EventBridge Scheduler の
// 両方から呼ばれる。取り違えると、スケジューラの起動要求が署名検証で弾かれて
// 稼働日に本番が上がらない、あるいは Discord のリクエストで
// ワークフローが暴発する。

import (
	"context"
	"encoding/json"
	"testing"

	"Backend/internal/services/discord"
)

func TestSchedulerEvent_Discordのペイロードと区別できる(t *testing.T) {
	// Discord の Interaction には source フィールドが無い。
	// ALB 経由のリクエスト本体（base64 or 生JSON）が source を持たないことを確認する。
	albPayload := `{"httpMethod":"POST","headers":{"x-signature-ed25519":"abc"},"body":"{\"type\":1}","isBase64Encoded":false}`

	var ev schedulerEvent
	if err := json.Unmarshal([]byte(albPayload), &ev); err != nil {
		t.Fatalf("パースに失敗: %v", err)
	}
	if ev.Source == schedulerSource {
		t.Error("ALB のリクエストがスケジューラ起動と誤判定される")
	}
}

func TestSchedulerEvent_スケジューラのペイロードを認識する(t *testing.T) {
	payload := `{"source":"prod-uptime-scheduler","workflow":"prod-uptime-scheduler.yml"}`

	var ev schedulerEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatalf("パースに失敗: %v", err)
	}
	if ev.Source != schedulerSource {
		t.Errorf("source を認識できない: %q", ev.Source)
	}
	if ev.Workflow != "prod-uptime-scheduler.yml" {
		t.Errorf("workflow を認識できない: %q", ev.Workflow)
	}
}

// ディスパッチャ未設定は成功にしない。
//
// 成功扱いにすると「起動しなかったのに誰も気づかない」状態に戻る。
// それがこのIssueで直している問題そのもの。
func TestHandleScheduled_未設定ならエラーにする(t *testing.T) {
	// token/repo 未設定の dispatcher は Enabled() が false になり、
	// DispatchWorkflow は (false, nil) を返す。
	t.Setenv("GITHUB_DISPATCH_TOKEN", "")
	t.Setenv("GITHUB_DISPATCH_REPO", "")

	_, err := handleScheduled(context.Background(), discord.NewWorkflowDispatcherFromEnv(), schedulerEvent{
		Source:   schedulerSource,
		Workflow: "prod-uptime-scheduler.yml",
	})
	if err == nil {
		t.Error("未設定を成功扱いにしてはいけない（起動しなくても気づけなくなる）")
	}
}
