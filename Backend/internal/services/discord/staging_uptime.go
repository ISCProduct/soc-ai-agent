package discord

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ステージングの起動状態（#1249）。
//
// 本番と違い日付リストは持たない。ステージングは展示会運用ではなく
// 開発用なので、「今起動しているか」だけを明示的に切り替える。
// auto を作らないのは、日付リストが無い以上 auto と on が同義になり、
// どちらを押したのか分からなくなるため。
const (
	StagingStateOn  = "on"
	StagingStateOff = "off"
)

// defaultStagingStateParameter は起動状態を持つSSMパラメータ。
//
// 未設定のときは on として扱う。パラメータを作る前から
// ステージングは動いているので、「無い＝止める」にすると
// この機能を入れた瞬間に環境が落ちる。
const defaultStagingStateParameter = "/soc-app/staging-uptime"

// StagingUptimeService はステージングの起動状態をSSMで管理する。
type StagingUptimeService struct {
	client        ssmAPI
	parameterName string
}

// NewStagingUptimeServiceFromEnv はAWSデフォルトクレデンシャルチェーンで構築する。
func NewStagingUptimeServiceFromEnv(ctx context.Context) (*StagingUptimeService, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	name := os.Getenv("STAGING_UPTIME_SSM_PARAMETER")
	if name == "" {
		name = defaultStagingStateParameter
	}
	return &StagingUptimeService{client: ssm.NewFromConfig(cfg), parameterName: name}, nil
}

// ParseStagingState は on / off のみ受理する（大文字小文字と前後空白は無視）。
func ParseStagingState(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case StagingStateOn:
		return StagingStateOn, nil
	case StagingStateOff:
		return StagingStateOff, nil
	default:
		return "", fmt.Errorf("状態は on / off のいずれかを指定してください")
	}
}

// GetState は現在の起動状態を返す。未設定・未知の値なら on。
//
// 判断できないときに off へ倒すと、ステージングが理由不明で落ちる。
// 「動いている」方を既定にして、止めるのは明示的な指示があったときだけにする。
func (s *StagingUptimeService) GetState(ctx context.Context) (string, error) {
	out, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(s.parameterName)})
	if err != nil {
		var notFound *ssmtypes.ParameterNotFound
		if errors.As(err, &notFound) {
			return StagingStateOn, nil
		}
		return "", fmt.Errorf("SSM parameter取得に失敗しました: %w", err)
	}
	state, parseErr := ParseStagingState(aws.ToString(out.Parameter.Value))
	if parseErr != nil {
		return StagingStateOn, nil
	}
	return state, nil
}

// SetState は起動状態を設定する。
func (s *StagingUptimeService) SetState(ctx context.Context, value string) error {
	normalized, err := ParseStagingState(value)
	if err != nil {
		return err
	}
	_, err = s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.parameterName),
		Value:     aws.String(normalized),
		Type:      ssmtypes.ParameterTypeString,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("SSM parameter更新に失敗しました: %w", err)
	}
	return nil
}
