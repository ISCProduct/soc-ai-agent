package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Handler は Discord Interactions（HTTP方式）を処理する。
//
// HTTPフレームワークに依存しない。受け口は Lambda Function URL だが、
// ロジックをそこに書くとテストが Lambda のイベント型に引きずられるため分けている。
//
// 受け口を backend（staging上）から Lambda へ移したのは、staging がデプロイ後
// 1時間で自動停止し、その間 /prod で本番を起動できなくなっていたため。
// 本番の起動/停止コマンドの受け口が、止まる環境に乗っていてはいけない。
type Handler struct {
	uptime        *UptimeService
	staging       StagingStateSetter
	dispatcher    *WorkflowDispatcher
	publicKey     string
	allowedRoleID string
}

// StagingStateSetter はステージングの起動状態の書き込み面（#1249）。
// テストで差し替えるために切っている。
type StagingStateSetter interface {
	SetState(ctx context.Context, value string) error
}

func NewHandler(uptime *UptimeService, staging StagingStateSetter, dispatcher *WorkflowDispatcher, publicKey, allowedRoleID string) *Handler {
	return &Handler{
		uptime:        uptime,
		staging:       staging,
		dispatcher:    dispatcher,
		publicKey:     publicKey,
		allowedRoleID: allowedRoleID,
	}
}

// MaxBodyBytes はDiscord Interactionペイロードの許容上限。
// 実際のペイロード(モーダル1入力分)は数百バイト程度なので十分な余裕を持たせつつ、
// 署名検証前の無制限読み取りによるDoSを防ぐ。
const MaxBodyBytes = 1 << 20 // 1MiB

// applyTimeout はSSM書き込みとワークフロー起動に許す時間。
//
// Discordは3秒以内に応答が無いと「アプリケーションは時間内に応答しませんでした」
// と表示する。backend時代は deferred(type=5) を返して goroutine で後追いしていたが、
// Lambdaはレスポンスを返すと実行環境が凍結されるため後追いが走らない。
// 実測でSSM+dispatchは1秒程度なので同期で処理し、万一遅れても打ち切って
// 「次の毎時実行で反映される」と伝える（SSMに書けていれば機能は成立する）。
const applyTimeout = 2500 * time.Millisecond

// Handle は署名検証からコマンド処理までを行い、返すべきHTTPステータスと本文を返す。
// 署名検証に失敗した場合は本文なしで401を返す。
func (h *Handler) Handle(ctx context.Context, body []byte, signature, timestamp string) (int, *InteractionResponse) {
	if len(body) > MaxBodyBytes {
		return http.StatusRequestEntityTooLarge, nil
	}
	if h.publicKey == "" || !VerifySignature(h.publicKey, signature, timestamp, body) {
		return http.StatusUnauthorized, nil
	}

	var interaction Interaction
	if err := json.Unmarshal(body, &interaction); err != nil {
		return http.StatusBadRequest, nil
	}

	switch interaction.Type {
	case InteractionTypePing:
		return http.StatusOK, &InteractionResponse{Type: ResponseTypePong}
	case InteractionTypeApplicationCommand:
		return http.StatusOK, h.handleCommand(ctx, &interaction)
	case InteractionTypeModalSubmit:
		return http.StatusOK, h.handleModalSubmit(ctx, &interaction)
	default:
		return http.StatusOK, ephemeral("未対応の操作です。")
	}
}

func (h *Handler) handleCommand(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if interaction.Data == nil {
		return ephemeral("不明なコマンドです。")
	}

	switch interaction.Data.Name {
	case CommandNameProdUptimeList:
		return h.listDates(ctx)
	case CommandNameProd:
		return h.setProdOverride(ctx, interaction)
	case CommandNameStaging:
		return h.setStagingState(ctx, interaction)
	case CommandNameProdUptime:
		if !h.hasAllowedRole(interaction) {
			return ephemeral("このコマンドを実行する権限がありません。")
		}
		return dateInputModal()
	default:
		return ephemeral("不明なコマンドです。")
	}
}

// setProdOverride は本番の起動状態を手動で切り替える（/prod state:on|off|auto）。
//
// ECSを直接叩かずSSMのオーバーライドを書くのは、prod-uptime-scheduler.yml が毎時
// 日付リストと照合して desired_count を上書きするため。直接起動しても最大1時間で
// 元に戻され、「Discordで起動したのに落ちている」状態になる。
func (h *Handler) setProdOverride(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if !h.hasAllowedRole(interaction) {
		return ephemeral("このコマンドを実行する権限がありません。")
	}

	state, err := ParseOverride(FindOptionString(interaction.Data.Options, OptionNameState))
	if err != nil {
		return ephemeral(err.Error())
	}
	if h.uptime == nil {
		return ephemeral("現在この機能は利用できません(未設定)。")
	}

	// 全ユーザーに影響する操作なので、誰がいつ実行したかを残す。
	// 「本番が落ちている、誰が止めたのか」を後から追えるようにする。
	log.Printf("[Discord] /prod state=%s by %s", state, interaction.ActorLabel())

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	if err := h.uptime.SetOverride(ctx, state); err != nil {
		log.Printf("[Discord] prod override set error: %v", err)
		return ephemeral(userFacingErrorMessage(err))
	}

	// 日付追加(/prod-uptime)と違い本番の稼働そのものを変えるため、
	// ephemeralにせずチャンネルに残す。
	return &InteractionResponse{
		Type: ResponseTypeChannelMessageWithSource,
		Data: &InteractionResponseData{
			Content: overrideAppliedMessage(state) + h.dispatchNote(ctx),
		},
	}
}

// setStagingState は /staging state:on|off を処理する（#1249）。
//
// 本番(/prod)と違い日付リストが無いので、指定はそのまま起動状態になる。
func (h *Handler) setStagingState(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if !h.hasAllowedRole(interaction) {
		return ephemeral("このコマンドを実行する権限がありません。")
	}

	state, err := ParseStagingState(FindOptionString(interaction.Data.Options, OptionNameState))
	if err != nil {
		return ephemeral(err.Error())
	}
	if h.staging == nil {
		return ephemeral("現在この機能は利用できません(未設定)。")
	}

	// 開発チーム全体に影響するので、誰がいつ止めたかを残す
	log.Printf("[Discord] /staging state=%s by %s", state, interaction.ActorLabel())

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	if err := h.staging.SetState(ctx, state); err != nil {
		log.Printf("[Discord] staging state set error: %v", err)
		return ephemeral(userFacingErrorMessage(err))
	}

	return &InteractionResponse{
		Type: ResponseTypeChannelMessageWithSource,
		Data: &InteractionResponseData{
			Content: stagingStateAppliedMessage(state) + h.dispatchNoteFor(ctx, StagingUptimeWorkflowFile),
		},
	}
}

// listDates は登録済み日付一覧を返す(閲覧専用、ロール制限なし)。
func (h *Handler) listDates(ctx context.Context) *InteractionResponse {
	if h.uptime == nil {
		return ephemeral("現在この機能は利用できません(未設定)。")
	}

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	dates, err := h.uptime.ListDates(ctx)
	if err != nil {
		// SSMのエラー内容(ARN等の内部情報を含みうる)はDiscordへ返さずログにのみ残す。
		log.Printf("[Discord] prod-uptime list error: %v", err)
		return ephemeral("日付一覧の取得に失敗しました。時間を置いて再度お試しください。")
	}

	content := "本番終日起動の登録済み日付: " + joinDates(dates)

	// 手動オーバーライド中は日付リストが効かないため、一覧だけ見て
	// 「今日は載っていないから止まっているはず」と誤解しないよう併記する。
	// 取得できなかった場合も黙って省略しない。表示が無いことを
	// 「auto に戻っている」と読まれると、on 固定のまま課金が続く。
	override, overrideErr := h.uptime.GetOverride(ctx)
	switch {
	case overrideErr != nil:
		log.Printf("[Discord] prod override get error: %v", overrideErr)
		content += "\n⚠️ 現在の設定を取得できませんでした(固定中かどうか不明です)。"
	case override != OverrideAuto:
		content += "\n⚠️ 現在 /prod で「" + overrideLabel(override) + "」に固定されています(日付リストは無視されます)。"
	default:
		content += "\n現在の設定: 日付リストに従う(auto)"
	}

	return ephemeral(content)
}

func (h *Handler) handleModalSubmit(ctx context.Context, interaction *Interaction) *InteractionResponse {
	if interaction.Data == nil || interaction.Data.CustomID != ModalCustomIDProdUptime {
		return ephemeral("不明な操作です。")
	}
	// コマンド実行時と同じ権限を、モーダル送信時にも再検証する
	// （モーダル表示後に権限が失効するケースを考慮）。
	if !h.hasAllowedRole(interaction) {
		return ephemeral("このコマンドを実行する権限がありません。")
	}

	date, err := ParseDate(FindComponentValue(interaction.Data.Components, TextInputCustomIDDate))
	if err != nil {
		return ephemeral(err.Error())
	}
	if h.uptime == nil {
		return ephemeral("現在この機能は利用できません(未設定)。")
	}

	ctx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	dates, err := h.uptime.AddDate(ctx, date)
	if err != nil {
		log.Printf("[Discord] prod-uptime add error: %v", err)
		return ephemeral(userFacingErrorMessage(err))
	}
	return ephemeral("✅ " + date + " を本番終日起動の対象日に追加しました。\n登録済みの日付: " + joinDates(dates))
}

// dispatchNote は即時反映の起動結果を利用者向けの一文にする。
//
// オーバーライドは書けているので、即時反映に失敗しても機能自体は成立する
// (次の毎時実行で反映される)。その旨をユーザーへ返す。
func (h *Handler) dispatchNote(ctx context.Context) string {
	dispatched, err := h.dispatcher.Dispatch(ctx)
	return dispatchNoteFromResult(dispatched, err, "反映を開始しました。完了まで数分かかります(本番DBの起動待ちを含む)。")
}

func (h *Handler) dispatchNoteFor(ctx context.Context, workflow string) string {
	dispatched, err := h.dispatcher.DispatchWorkflow(ctx, workflow)
	return dispatchNoteFromResult(dispatched, err, "反映を開始しました。起動は完了まで数分かかります。")
}

func dispatchNoteFromResult(dispatched bool, err error, okMessage string) string {
	switch {
	case err != nil:
		log.Printf("[Discord] dispatch error: %v", err)
		return "\n⚠️ 即時反映の起動に失敗しました。次の毎時実行(最大1時間後)で反映されます。"
	case dispatched:
		return "\n" + okMessage
	default:
		return "\n次の毎時実行(最大1時間後)で反映されます。"
	}
}

// StagingUptimeWorkflowFile はステージングの起動/停止を行うワークフロー。
const StagingUptimeWorkflowFile = "staging-uptime-scheduler.yml"

func (h *Handler) hasAllowedRole(interaction *Interaction) bool {
	if h.allowedRoleID == "" || interaction.Member == nil {
		return false
	}
	for _, r := range interaction.Member.Roles {
		if r == h.allowedRoleID {
			return true
		}
	}
	return false
}

// ephemeral は本人にしか見えないメッセージ応答を作る。
func ephemeral(content string) *InteractionResponse {
	return &InteractionResponse{
		Type: ResponseTypeChannelMessageWithSource,
		Data: &InteractionResponseData{Content: content, Flags: EphemeralFlag},
	}
}

// dateInputModal は日付追加用のモーダルを作る。
func dateInputModal() *InteractionResponse {
	return &InteractionResponse{
		Type: ResponseTypeModal,
		Data: &InteractionResponseData{
			CustomID: ModalCustomIDProdUptime,
			Title:    "本番を終日起動する日付を追加",
			Components: []Component{
				{
					Type: ComponentTypeActionRow,
					Components: []Component{
						{
							Type:        ComponentTypeTextInput,
							CustomID:    TextInputCustomIDDate,
							Style:       TextInputStyleShort,
							Label:       "日付 (YYYY-MM-DD, JST)",
							Placeholder: "2026-09-01",
							Required:    true,
						},
					},
				},
			},
		},
	}
}

// overrideLabel は状態の表示名。
func overrideLabel(state string) string {
	switch state {
	case OverrideOn:
		return "常時起動"
	case OverrideOff:
		return "常時停止"
	default:
		return "日付リストに従う"
	}
}

// overrideAppliedMessage は設定した状態をそのまま読める文言にする。
func overrideAppliedMessage(state string) string {
	switch state {
	case OverrideOn:
		return "✅ 本番を「常時起動」に設定しました。日付リストに関係なく起動し続けます。"
	case OverrideOff:
		return "🛑 本番を「常時停止」に設定しました。日付リストに関係なく停止します。"
	default:
		return "🔄 本番を「日付リストに従う」に戻しました。"
	}
}

func stagingStateAppliedMessage(state string) string {
	if state == StagingStateOn {
		return "ステージング環境を **起動** に設定しました。"
	}
	return "ステージング環境を **停止** に設定しました。"
}

// userFacingErrorMessage はDiscordへそのまま見せてよいエラーメッセージを判定する。
// uptimeServiceの入力検証エラー(日付形式・過去日等)は日本語の固定文言のみでラップされて
// いないため安全だが、SSMアクセス失敗等の内部エラーは%wでラップされ、ARN等の内部情報を
// 含みうるため一般的な文言に差し替える。
func userFacingErrorMessage(err error) string {
	if errors.Unwrap(err) != nil {
		return "処理に失敗しました。時間を置いて再度お試しください。"
	}
	return err.Error()
}

// joinDatesMaxChars はjoinDatesが返す文字列の上限。Discordのメッセージ本文は最大2000文字
// (content フィールド単体の上限で、プレフィックス文言と合わせても収まるよう余裕を持たせる)。
const joinDatesMaxChars = 1500

func joinDates(dates []string) string {
	if len(dates) == 0 {
		return "(なし)"
	}
	out := dates[0]
	for i, d := range dates[1:] {
		if len(out) > joinDatesMaxChars {
			remaining := len(dates) - 1 - i
			out += fmt.Sprintf(", ...(他%d件)", remaining)
			return out
		}
		out += ", " + d
	}
	return out
}
