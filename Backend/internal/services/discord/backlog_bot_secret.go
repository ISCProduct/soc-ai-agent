package discord

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const (
	envBacklogBotSecretID       = "DISCORD_BACKLOG_BOT_SECRET_ID"
	backlogBotSecretPlaceholder = "replace-via-cli"
)

// fetchBacklogBotSecret はテストが差し替える。本番は Secrets Manager を読む。
var fetchBacklogBotSecret = fetchBacklogBotSecretAWS

// ResolveBacklogAPIKey は Bot 用キーを返す。
// DISCORD_BACKLOG_BOT_SECRET_ID があるときはそのシークレットだけを使い、
// 環境変数 BACKLOG_API_KEY（個人のキー）には戻らない。
// シークレットが空、または未設定の印なら空文字を返す。
func ResolveBacklogAPIKey(ctx context.Context) (string, error) {
	if id := strings.TrimSpace(os.Getenv(envBacklogBotSecretID)); id != "" {
		key, err := fetchBacklogBotSecret(ctx, id)
		if err != nil {
			return "", err
		}
		key = strings.TrimSpace(key)
		if key == "" || key == backlogBotSecretPlaceholder {
			return "", nil
		}
		return key, nil
	}
	return os.Getenv("BACKLOG_API_KEY"), nil
}

func fetchBacklogBotSecretAWS(ctx context.Context, id string) (string, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("backlog bot secret の設定を読めません: %w", err)
	}
	out, err := secretsmanager.NewFromConfig(cfg).GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(id),
	})
	if err != nil {
		return "", fmt.Errorf("backlog bot の API キーを読めません: %w", err)
	}
	if out.SecretString == nil {
		return "", fmt.Errorf("backlog bot の API キーが文字列でありません")
	}
	return *out.SecretString, nil
}
