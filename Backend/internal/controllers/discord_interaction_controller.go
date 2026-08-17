package controllers

import (
	"Backend/internal/services/discord"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

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

// Interactions POST /api/discord/interactions
func (c *DiscordInteractionController) Interactions(ctx echo.Context) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to read body")
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
	if interaction.Data == nil || interaction.Data.Name != discord.CommandNameProdUptime {
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

	dates, err := c.uptimeService.AddDate(ctx.Request().Context(), date)
	if err != nil {
		log.Printf("[Discord] prod-uptime add error: %v", err)
		return ctx.JSON(http.StatusOK, discord.InteractionResponse{
			Type: discord.ResponseTypeChannelMessageWithSource,
			Data: &discord.InteractionResponseData{Content: err.Error(), Flags: discord.EphemeralFlag},
		})
	}

	return ctx.JSON(http.StatusOK, discord.InteractionResponse{
		Type: discord.ResponseTypeChannelMessageWithSource,
		Data: &discord.InteractionResponseData{
			Content: "✅ " + date + " を本番終日起動の対象日に追加しました。\n登録済みの日付: " + joinDates(dates),
			Flags:   discord.EphemeralFlag,
		},
	})
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

func joinDates(dates []string) string {
	if len(dates) == 0 {
		return "(なし)"
	}
	out := dates[0]
	for _, d := range dates[1:] {
		out += ", " + d
	}
	return out
}
