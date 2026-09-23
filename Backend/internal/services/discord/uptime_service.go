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

// dateLayout は日付リストと復帰日で共通の書式。JST の暦日で扱う。
const dateLayout = "2006-01-02"

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

// ParseOverride は on / off / auto と、復帰日付き(on:YYYY-MM-DD / off:YYYY-MM-DD)を
// 受理する（大文字小文字と前後空白は無視）。
//
// 復帰日は「この日(JST)になったら auto へ戻す」という意味。期限の無い off は
// 戻し忘れると起動日が丸ごと潰れる。実際 2026-09-12 に設定された off が放置され、
// 9/24 の起動日を潰しかけたうえ、本番デプロイのマイグレーションが3回失敗していた。
//
//	off:2026-09-24  -> 9/23 までは停止、9/24 になったら日付リストに従う
func ParseOverride(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	state, expiry, hasExpiry := strings.Cut(normalized, ":")

	switch state {
	case OverrideOn, OverrideOff:
	case OverrideAuto, "":
		if hasExpiry {
			// auto は既定状態なので、そこへ戻す日付を指定しても意味が無い。
			// 受理すると「設定したのに何も変わらない」誤解を生む。
			return "", fmt.Errorf("auto に復帰日は指定できません（on:YYYY-MM-DD / off:YYYY-MM-DD の形で指定してください）")
		}
		return OverrideAuto, nil
	default:
		return "", fmt.Errorf("状態は on / off / auto のいずれかを指定してください")
	}

	if !hasExpiry {
		return state, nil
	}
	if !dateOnlyPattern.MatchString(expiry) {
		return "", fmt.Errorf("復帰日は YYYY-MM-DD の形式で指定してください（例: %s:2026-09-24）", state)
	}
	if _, err := time.Parse(dateLayout, expiry); err != nil {
		return "", fmt.Errorf("復帰日が実在しない日付です: %s", expiry)
	}
	return state + ":" + expiry, nil
}

// ResolveOverride は保存値と今日の日付(JST)から、実際に効かせる状態を返す。
//
// 復帰日に達していれば auto を返す。判定は prod-uptime-scheduler.yml 側にも
// 同じものがある（あちらが desired_count を決める最終地点のため）。
// YYYY-MM-DD の辞書順比較で日付順と一致するので、両方とも文字列比較で揃えている。
func ResolveOverride(raw, todayJST string) string {
	state, expiry, hasExpiry := strings.Cut(strings.ToLower(strings.TrimSpace(raw)), ":")
	if state != OverrideOn && state != OverrideOff {
		return OverrideAuto
	}
	if !hasExpiry {
		return state
	}
	if !dateOnlyPattern.MatchString(expiry) {
		// 壊れた値で本番を起動しっぱなしにする/落とすより、日付リストに従う方が安全。
		return OverrideAuto
	}
	if todayJST >= expiry {
		return OverrideAuto
	}
	return state
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
	if _, err := time.Parse(dateLayout, trimmed); err != nil {
		return "", fmt.Errorf("存在しない日付です: %s", trimmed)
	}
	return trimmed, nil
}

// AddDate は指定日をリストへ追加する（べき等）。過去日は追加を拒否する。
func (s *UptimeService) AddDate(ctx context.Context, date string) ([]string, error) {
	today := time.Now().In(jst()).Format(dateLayout)
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

// todayJST は JST の今日の日付を YYYY-MM-DD で返す。
// 日付リストと復帰日の判定はどちらも JST の暦日で行う。
func todayJST() string {
	return time.Now().In(jst()).Format(dateLayout)
}

func jst() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}
