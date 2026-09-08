package discord

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	defaultParameterName         = "/soc-app/prod-uptime-dates"
	defaultOverrideParameterName = "/soc-app/prod-uptime-override"
)

// 手動オーバーライドの値。日付リストより優先して本番の起動状態を決める。
const (
	OverrideOn   = "on"   // 日付に関係なく起動し続ける
	OverrideOff  = "off"  // 日付に関係なく停止する
	OverrideAuto = "auto" // オーバーライドせず、日付リストに従う（既定）
)

var dateOnlyPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// UptimeService は本番の「指定日終日起動」日付リストをSSM Parameter Storeで管理する。
// #881台のインフラ方針(docs/architecture/infra-decision-oci-stg-aws-prod.md)の
// 「指定日リスト」をSSM Parameterに持つ実装。
// ssmAPI はUptimeServiceが使うSSM操作。テストで差し替えるために切っている。
type ssmAPI interface {
	GetParameter(ctx context.Context, in *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	PutParameter(ctx context.Context, in *ssm.PutParameterInput, opts ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

type UptimeService struct {
	client                ssmAPI
	parameterName         string
	overrideParameterName string
	// ponytail: read-modify-writeの排他はプロセス内mutexのみ。
	// staging EC2は単一インスタンス運用(#829)のため実害はないが、複数インスタンス化する場合は
	// SSM単体にCAS機構が無いため、DynamoDB等を使った分散ロックへの置き換えが必要。
	mu sync.Mutex
}

// NewUptimeServiceFromEnv はAWSデフォルトクレデンシャルチェーン(IAMロール等)でクライアントを構築する。
func NewUptimeServiceFromEnv(ctx context.Context) (*UptimeService, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	name := os.Getenv("PROD_UPTIME_SSM_PARAMETER")
	if name == "" {
		name = defaultParameterName
	}
	overrideName := os.Getenv("PROD_UPTIME_OVERRIDE_SSM_PARAMETER")
	if overrideName == "" {
		overrideName = defaultOverrideParameterName
	}
	return &UptimeService{
		client:                ssm.NewFromConfig(cfg),
		parameterName:         name,
		overrideParameterName: overrideName,
	}, nil
}

// ParseOverride は on / off / auto のみ受理する（大文字小文字と前後空白は無視）。
func ParseOverride(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case OverrideOn:
		return OverrideOn, nil
	case OverrideOff:
		return OverrideOff, nil
	case OverrideAuto, "":
		return OverrideAuto, nil
	default:
		return "", fmt.Errorf("状態は on / off / auto のいずれかを指定してください")
	}
}

// GetOverride は現在の手動オーバーライドを返す。未設定なら auto。
func (s *UptimeService) GetOverride(ctx context.Context) (string, error) {
	out, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(s.overrideParameterName)})
	if err != nil {
		var notFound *ssmtypes.ParameterNotFound
		if errors.As(err, &notFound) {
			return OverrideAuto, nil
		}
		return "", fmt.Errorf("SSM parameter取得に失敗しました: %w", err)
	}
	// 手で書き換えられて未知の値になっていても、勝手に on/off と解釈せず auto に倒す。
	// 誤って本番を起動しっぱなしにする/落とすより、日付リストどおりに動く方が安全。
	value, err := ParseOverride(aws.ToString(out.Parameter.Value))
	if err != nil {
		return OverrideAuto, nil
	}
	return value, nil
}

// SetOverride は手動オーバーライドを設定する。値は ParseOverride 済みであること。
func (s *UptimeService) SetOverride(ctx context.Context, value string) error {
	normalized, err := ParseOverride(value)
	if err != nil {
		return err
	}
	_, err = s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.overrideParameterName),
		Value:     aws.String(normalized),
		Type:      ssmtypes.ParameterTypeString,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("SSM parameter更新に失敗しました: %w", err)
	}
	return nil
}

// ParseDate は "YYYY-MM-DD" 形式のみ受理する（JSTの暦日として扱う）。
func ParseDate(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if !dateOnlyPattern.MatchString(trimmed) {
		return "", fmt.Errorf("日付はYYYY-MM-DD形式で入力してください（例: 2026-09-01）")
	}
	if _, err := time.Parse("2006-01-02", trimmed); err != nil {
		return "", fmt.Errorf("存在しない日付です: %s", trimmed)
	}
	return trimmed, nil
}

// AddDate は指定日をリストへ追加する（べき等）。過去日は追加を拒否する。
func (s *UptimeService) AddDate(ctx context.Context, date string) ([]string, error) {
	today := time.Now().In(jst()).Format("2006-01-02")
	if date < today {
		return nil, fmt.Errorf("過去の日付は指定できません（今日: %s）", today)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dates, err := s.listDates(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range dates {
		if d == date {
			return dates, nil // 既に登録済み
		}
	}
	dates = append(dates, date)
	sort.Strings(dates)

	value := strings.Join(dates, ",")
	_, err = s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.parameterName),
		Value:     aws.String(value),
		Type:      ssmtypes.ParameterTypeString,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("SSM parameter更新に失敗しました: %w", err)
	}
	return dates, nil
}

// ListDates は登録済みの日付一覧を返す(閲覧専用、ロール制限なしのコマンドから利用)。
func (s *UptimeService) ListDates(ctx context.Context) ([]string, error) {
	return s.listDates(ctx)
}

func (s *UptimeService) listDates(ctx context.Context) ([]string, error) {
	out, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(s.parameterName)})
	if err != nil {
		var notFound *ssmtypes.ParameterNotFound
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("SSM parameter取得に失敗しました: %w", err)
	}
	value := aws.ToString(out.Parameter.Value)
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	dates := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			dates = append(dates, p)
		}
	}
	return dates, nil
}

func jst() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}
