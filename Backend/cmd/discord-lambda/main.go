// Command discord-lambda は Discord Interactions Endpoint を提供する Lambda。
//
// 受け口を backend（staging上）からここへ移した。staging はデプロイ後1時間で
// 自動停止するため、本番を起動する /prod の受け口がその間使えず、
// 「アプリケーションは時間内に応答しませんでした」になっていた。
// 本番の起動/停止を指示する口が、止まる環境に乗っていてはいけない。
//
// 公開は既存の staging ALB のリスナールール経由で行う。ALB は EC2 とは独立して
// 常時稼働しているので、staging が停止していてもこの受け口は生きている。
// 既に払っている ALB を使うため追加コストが無く、URL も
// https://api-stg.shukatsu-ai.jp/api/discord/interactions のまま変わらない。
//
// Lambda Function URL は設定が正当でも 403 (AccessDeniedException) から
// 抜けられなかった（AuthType=NONE・リソースポリシー正当・DNS/TLS正常・
// 関数本体は invoke で動作確認済み、URL再作成でも解消せず）。
// アカウント側の制限が疑われるが特定できないため ALB 経由にしている。
//
// SSM も GitHub API もパブリックエンドポイントなので VPC には入れない
// （NAT Gateway が不要になる）。
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"

	"Backend/internal/services/discord"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	ctx := context.Background()

	uptime, err := discord.NewUptimeServiceFromEnv(ctx)
	if err != nil {
		// 起動時に落としてよい。SSMを読めない状態で受け付けても
		// 「失敗しました」を返すだけで、原因がDiscord側からは分からない。
		log.Fatalf("UptimeService の初期化に失敗しました: %v", err)
	}
	staging, err := discord.NewStagingUptimeServiceFromEnv(ctx)
	if err != nil {
		log.Fatalf("StagingUptimeService の初期化に失敗しました: %v", err)
	}

	handler := discord.NewHandler(
		uptime,
		staging,
		discord.NewWorkflowDispatcherFromEnv(),
		os.Getenv("DISCORD_PUBLIC_KEY"),
		os.Getenv("DISCORD_ALLOWED_ROLE_ID"),
	)

	lambda.Start(newLambdaHandler(handler, discord.NewWorkflowDispatcherFromEnv()))
}

// schedulerEvent は EventBridge Scheduler から渡す固定ペイロード（#1388）。
//
// GitHub ホストの scheduled workflow はベストエフォートで、実測では
// 毎時のはずの prod-uptime-scheduler が平均4時間・最大5.5時間空いていた。
// 稼働日の朝に本番が起動していない事故につながるため、発火は AWS 側の
// スケジューラに任せ、ここから workflow_dispatch で既存ワークフローを叩く。
//
// 起動手順（RDS待ち、chroma→rag-review→backend→frontend の順序、
// オートスケーリング下限の同期）はワークフローが持っている。Go 側に書き直すと
// 二重管理になり、片方だけ直して本番が中途半端に起動する事故になる。
type schedulerEvent struct {
	Source   string `json:"source"`
	Workflow string `json:"workflow"`
}

// schedulerSource は EventBridge からの呼び出しであることを示す値。
// Discord の Interaction ペイロードと取り違えないよう、固定文字列で判定する。
const schedulerSource = "prod-uptime-scheduler"

// newLambdaHandler は ALB からの Discord Interaction と、
// EventBridge Scheduler からの起動要求の両方を受ける。
//
// 入口を分けずに1つの Lambda で受けるのは、本番を起動する口を増やさないため。
// ALB 経由の受け口は staging が停止していても生きている（既存の構成）。
func newLambdaHandler(h *discord.Handler, dispatcher *discord.WorkflowDispatcher) func(context.Context, json.RawMessage) (any, error) {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		// EventBridge Scheduler からの呼び出しかを先に判定する。
		// ALB のリクエストには source フィールドが無いので取り違えない。
		var ev schedulerEvent
		if err := json.Unmarshal(raw, &ev); err == nil && ev.Source == schedulerSource {
			return handleScheduled(ctx, dispatcher, ev)
		}

		var req events.ALBTargetGroupRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			log.Printf("[Discord] event decode error: %v", err)
			return textResponse(http.StatusBadRequest), nil
		}
		return handleALB(ctx, h, req)
	}
}

// handleScheduled はワークフローを起動する。
//
// 失敗しても Lambda をエラー終了させる。EventBridge のリトライに任せられるうえ、
// 成功扱いにすると「起動しなかったのに誰も気づかない」状態に戻る。
func handleScheduled(ctx context.Context, dispatcher *discord.WorkflowDispatcher, ev schedulerEvent) (any, error) {
	dispatched, err := dispatcher.DispatchWorkflow(ctx, ev.Workflow)
	if err != nil {
		log.Printf("[Scheduler] workflow dispatch failed: %v", err)
		return nil, err
	}
	if !dispatched {
		// トークン未設定。設定漏れに気づけるようエラーにする。
		log.Printf("[Scheduler] workflow dispatch skipped: dispatcher not configured")
		return nil, errDispatcherNotConfigured
	}
	log.Printf("[Scheduler] workflow dispatched: %s", ev.Workflow)
	return map[string]string{"status": "dispatched", "workflow": ev.Workflow}, nil
}

var errDispatcherNotConfigured = errors.New("workflow dispatcher is not configured")

func handleALB(ctx context.Context, h *discord.Handler, req events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	body, err := decodeBody(req)
	if err != nil {
		log.Printf("[Discord] body decode error: %v", err)
		return textResponse(http.StatusBadRequest), nil
	}

	// ALB はヘッダ名を小文字にして渡す。
	status, resp := h.Handle(ctx, body,
		req.Headers["x-signature-ed25519"],
		req.Headers["x-signature-timestamp"],
	)
	if resp == nil {
		return textResponse(status), nil
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[Discord] response marshal error: %v", err)
		return textResponse(http.StatusInternalServerError), nil
	}

	return events.ALBTargetGroupResponse{
		StatusCode:        status,
		StatusDescription: statusDescription(status),
		Headers:           map[string]string{"Content-Type": "application/json"},
		Body:              string(payload),
	}, nil
}

// errBodyTooLarge は署名検証前に打ち切った場合のエラー。
var errBodyTooLarge = errors.New("body too large")

func decodeBody(req events.ALBTargetGroupRequest) ([]byte, error) {
	// base64のデコード後は元より小さくなるので、デコード前の長さで判定してよい。
	// 署名検証前に大きなペイロードを展開しないためのガード。
	if len(req.Body) > discord.MaxBodyBytes {
		return nil, errBodyTooLarge
	}
	if !req.IsBase64Encoded {
		return []byte(req.Body), nil
	}
	return base64.StdEncoding.DecodeString(req.Body)
}

// textResponse は本文を持たない応答を作る。
// Discordへ内部情報を返さないよう、ステータスの標準文言だけを返す。
func textResponse(status int) events.ALBTargetGroupResponse {
	return events.ALBTargetGroupResponse{
		StatusCode:        status,
		StatusDescription: statusDescription(status),
		Headers:           map[string]string{"Content-Type": "text/plain; charset=utf-8"},
		Body:              http.StatusText(status),
	}
}

// statusDescription は ALB が要求する "200 OK" 形式の文字列を作る。
func statusDescription(status int) string {
	return strconv.Itoa(status) + " " + http.StatusText(status)
}
