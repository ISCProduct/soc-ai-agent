package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const discordAPIBase = "https://discord.com/api/v10"

var followupHTTPClient = &http.Client{Timeout: 10 * time.Second}

// EditOriginalResponse はtype=5(deferred)で即時応答した後、確定した内容で
// オリジナルメッセージを更新する。interaction token自体が認可情報を兼ねるため
// 追加の認証ヘッダは不要(Botトークンをバックエンドに持たせずに済む)。
func EditOriginalResponse(applicationID, token, content string) error {
	body, err := json.Marshal(map[string]any{"content": content})
	if err != nil {
		return fmt.Errorf("marshal followup body: %w", err)
	}

	url := fmt.Sprintf("%s/webhooks/%s/%s/messages/@original", discordAPIBase, applicationID, token)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build followup request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := followupHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send followup request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord followup returned status %d", resp.StatusCode)
	}
	return nil
}
