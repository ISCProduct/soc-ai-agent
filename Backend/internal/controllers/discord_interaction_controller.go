package controllers

import (
	"Backend/internal/services/discord"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

// DiscordInteractionController はDiscord Interactions Endpoint（HTTP方式）を処理する。
// 本番の「指定日は終日起動」日付リストを、Discordのスラッシュコマンド+モーダルから
// 登録できるようにする（docs/architecture/infra-decision-oci-stg-aws-prod.md の
// 「指定日リスト」運用を実現する入力口）。
type DiscordInteractionController struct {
	uptimeService *discord.UptimeService
	publicKey     string
	allowedRoleID string
}

func NewDiscordInteractionController(uptimeService *discord.UptimeService) *DiscordInteractionController {
	return &DiscordInteractionController{
		uptimeService: uptimeService,
		publicKey:     os.Getenv("DISCORD_PUBLIC_KEY"),
		allowedRoleID: os.Getenv("DISCORD_ALLOWED_ROLE_ID"),
	}
}

// maxInteractionBodyBytes はDiscord Interactionペイロードの許容上限。
// 実際のペイロード(モーダル1入力分)は数百バイト程度なので十分な余裕を持たせつつ、
// 署名検証前の無制限読み取りによるDoSを防ぐ。
const maxInteractionBodyBytes = 1 << 20 // 1MiB

// Interactions POST /api/discord/interactions
func (c *DiscordInteractionController) Interactions(ctx echo.Context) error {
	ctx.Request().Body = http.MaxBytesReader(ctx.Response(), ctx.Request().Body, maxInteractionBodyBytes)
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "request body too large")
	}

	signature := ctx.Request().Header.Get("X-Signature-Ed25519")
	timestamp := ctx.Request().Header.Get("X-Signature-Timestamp")
	if c.publicKey == "" || !discord.VerifySignature(c.publicKey, signature, timestamp, body) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid request signature")
	}

	var interaction discord.Interaction
	if err := json.Unmarshal(body, &interaction); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	switch interaction.Type {
	case discord.InteractionTypePing:
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{Type: discord.ResponseTypePong})

	case discord.InteractionTypeApplicationCommand:
		return c.handleCommand(ctx, &interaction)

	case discord.InteractionTypeModalSubmit:
		return c.handleModalSubmit(ctx, &interaction)

	default:
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "未対応の操作です。", Flags: discord.EphemeralFlag},
		})
	}
}

func (c *DiscordInteractionController) handleCommand(ctx echo.Context, interaction *discord.Interaction) error {
	if interaction.Data == nil {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "不明なコマンドです。", Flags: discord.EphemeralFlag},
		})
	}

	if interaction.Data.Name == discord.CommandNameProdUptimeList {
		return c.handleListCommand(interaction, ctx)
	}

	if interaction.Data.Name != discord.CommandNameProdUptime {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "不明なコマンドです。", Flags: discord.EphemeralFlag},
		})
	}
	if !c.hasAllowedRole(interaction) {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "このコマンドを実行する権限がありません。", Flags: discord.EphemeralFlag},
		})
	}

	return ctx.JSON(http.StatusOK, discord.InteractionResponse{
		Type: discord.ResponseTypeModal,
		Data: &discord.InteractionResponseData{
			CustomID: discord.ModalCustomIDProdUptime,
			Title:    "本番を終日起動する日付を追加",
			Components: []discord.Component{
				{
					Type: discord.ComponentTypeActionRow,
					Components: []discord.Component{
						{
							Type:        discord.ComponentTypeTextInput,
							CustomID:    discord.TextInputCustomIDDate,
							Style:       discord.TextInputStyleShort,
							Label:       "日付 (YYYY-MM-DD, JST)",
							Placeholder: "2026-09-01",
							Required:    true,
						},
					},
				},
			},
		},
	})
}

// handleListCommand は登録済み日付一覧を返す(閲覧専用、ロール制限なし)。
// SSM読み取りがDiscordの3秒応答制限を超える可能性があるため、handleModalSubmitと
// 同様にdeferred応答(type=5)を返し、実処理は非同期でフォローアップメッセージとして送る。
func (c *DiscordInteractionController) handleListCommand(interaction *discord.Interaction, ctx echo.Context) error {
	if c.uptimeService == nil {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "現在この機能は利用できません(未設定)。", Flags: discord.EphemeralFlag},
		})
	}

	applicationID, token := interaction.ApplicationID, interaction.Token
	go c.listDatesAndFollowUp(applicationID, token)

	return ctx.JSON(http.StatusOK, discord.InteractionResponse{
		Type: discord.ResponseTypeDeferredChannelMessageWithSource,
		Data: &discord.InteractionResponseData{Flags: discord.EphemeralFlag},
	})
}

func (c *DiscordInteractionController) listDatesAndFollowUp(applicationID, token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dates, err := c.uptimeService.ListDates(ctx)
	// SSMのエラー内容(ARN等の内部情報を含みうる)はDiscordへ返さずログにのみ残す。
	content := "本番終日起動の登録済み日付: " + joinDates(dates)
	if err != nil {
		log.Printf("[Discord] prod-uptime list error: %v", err)
		content = "日付一覧の取得に失敗しました。時間を置いて再度お試しください。"
	}

	if err := discord.EditOriginalResponse(applicationID, token, content); err != nil {
		log.Printf("[Discord] prod-uptime list followup error: %v", err)
	}
}

func (c *DiscordInteractionController) handleModalSubmit(ctx echo.Context, interaction *discord.Interaction) error {
	if interaction.Data == nil || interaction.Data.CustomID != discord.ModalCustomIDProdUptime {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "不明な操作です。", Flags: discord.EphemeralFlag},
		})
	}
	// コマンド実行時と同じ権限を、モーダル送信時にも再検証する
	// （モーダル表示後に権限が失効するケースを考慮）。
	if !c.hasAllowedRole(interaction) {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "このコマンドを実行する権限がありません。", Flags: discord.EphemeralFlag},
		})
	}

	raw := discord.FindComponentValue(interaction.Data.Components, discord.TextInputCustomIDDate)
	date, err := discord.ParseDate(raw)
	if err != nil {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: err.Error(), Flags: discord.EphemeralFlag},
		})
	}

	if c.uptimeService == nil {
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: "現在この機能は利用できません(未設定)。", Flags: discord.EphemeralFlag},
		})
	}

	// SSMへの読み書きがDiscordの3秒応答制限を超える可能性があるため、
	// 先にdeferred応答(type=5)を返し、実処理は非同期でフォローアップメッセージとして送る。
	applicationID, token := interaction.ApplicationID, interaction.Token
	go c.addDateAndFollowUp(applicationID, token, date)

	return ctx.JSON(http.StatusOK, discord.InteractionResponse{
		Type: discord.ResponseTypeDeferredChannelMessageWithSource,
		Data: &discord.InteractionResponseData{Flags: discord.EphemeralFlag},
	})
}

func (c *DiscordInteractionController) addDateAndFollowUp(applicationID, token, date string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dates, err := c.uptimeService.AddDate(ctx, date)
	content := "✅ " + date + " を本番終日起動の対象日に追加しました。\n登録済みの日付: " + joinDates(dates)
	if err != nil {
		log.Printf("[Discord] prod-uptime add error: %v", err)
		content = userFacingErrorMessage(err)
	}

	if err := discord.EditOriginalResponse(applicationID, token, content); err != nil {
		log.Printf("[Discord] prod-uptime followup error: %v", err)
	}
}

func (c *DiscordInteractionController) hasAllowedRole(interaction *discord.Interaction) bool {
	if c.allowedRoleID == "" || interaction.Member == nil {
		return false
	}
	for _, r := range interaction.Member.Roles {
		if r == c.allowedRoleID {
			return true
		}
	}
	return false
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
