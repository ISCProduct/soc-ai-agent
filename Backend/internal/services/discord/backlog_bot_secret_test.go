package discord

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolveBacklogAPIKey(t *testing.T) {
	orig := fetchBacklogBotSecret
	t.Cleanup(func() { fetchBacklogBotSecret = orig })

	t.Run("シークレットが無ければ環境変数を使う", func(t *testing.T) {
		t.Setenv(envBacklogBotSecretID, "")
		t.Setenv("BACKLOG_API_KEY", "personal-key")
		called := false
		fetchBacklogBotSecret = func(context.Context, string) (string, error) {
			called = true
			return "bot-key", nil
		}
		got, err := ResolveBacklogAPIKey(context.Background())
		if err != nil || got != "personal-key" || called {
			t.Fatalf("got %q err %v called %v", got, err, called)
		}
	})

	t.Run("シークレットがあれば個人キーは使わない", func(t *testing.T) {
		t.Setenv(envBacklogBotSecretID, "soc-stg/discord-backlog-bot")
		t.Setenv("BACKLOG_API_KEY", "personal-key")
		fetchBacklogBotSecret = func(_ context.Context, id string) (string, error) {
			if id != "soc-stg/discord-backlog-bot" {
				t.Errorf("secret id = %s", id)
			}
			return "bot-key", nil
		}
		got, err := ResolveBacklogAPIKey(context.Background())
		if err != nil || got != "bot-key" {
			t.Fatalf("got %q err %v", got, err)
		}
	})

	t.Run("未設定の印は空で個人キーに戻らない", func(t *testing.T) {
		t.Setenv(envBacklogBotSecretID, "soc-stg/discord-backlog-bot")
		t.Setenv("BACKLOG_API_KEY", "personal-key")
		fetchBacklogBotSecret = func(context.Context, string) (string, error) {
			return backlogBotSecretPlaceholder, nil
		}
		got, err := ResolveBacklogAPIKey(context.Background())
		if err != nil || got != "" {
			t.Fatalf("got %q err %v", got, err)
		}
	})

	t.Run("失敗にキーを含めない", func(t *testing.T) {
		t.Setenv(envBacklogBotSecretID, "soc-stg/discord-backlog-bot")
		t.Setenv("BACKLOG_API_KEY", "personal-key")
		fetchBacklogBotSecret = func(context.Context, string) (string, error) {
			return "", errors.New("AccessDeniedException")
		}
		got, err := ResolveBacklogAPIKey(context.Background())
		if err == nil || got != "" {
			t.Fatalf("got %q err %v", got, err)
		}
		if strings.Contains(err.Error(), "personal-key") || strings.Contains(err.Error(), "bot-key") {
			t.Fatalf("エラーにキーが含まれる: %v", err)
		}
	})
}
