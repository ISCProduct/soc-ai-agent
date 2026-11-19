package repositories

import (
	"Backend/internal/models"
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"strings"
	"time"

	"gorm.io/gorm"
)

type APICallLogRepository struct {
	db *gorm.DB
}

func NewAPICallLogRepository(db *gorm.DB) *APICallLogRepository {
	return &APICallLogRepository{db: db}
}

// Create はログを保存する。school_id は呼び出し時点の所属として利用者から引く。
// users.school_id を集計時に JOIN すると、所属が変わったときに過去の実績まで移動する。
// リクエストごとに所属を受け渡さず記録時に引くので、予算判定の有無によらず全経路に効く。
func (r *APICallLogRepository) Create(log *models.APICallLog) error {
	if log.UserID != nil && log.SchoolID == nil {
		log.SchoolID = schoolIDForLog(r.db, *log.UserID)
	}
	return r.db.Create(log).Error
}

// schoolIDForLog は利用者の所属校を返す。引けないときは nil を返し、利用ログの記録は止めない。
//
// 利用者がいないときと、DB の失敗で引けなかったときを分ける。後者は学校の利用額から
// 外れる（学校の上限だけを設定している場合はそのぶん上限の外になる）ので、ログに残して
// 気づけるようにする。
func schoolIDForLog(db *gorm.DB, userID uint) *uint {
	var row struct{ SchoolID *uint }
	if err := db.Model(&models.User{}).Select("school_id").Where("id = ?", userID).Take(&row).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			stdlog.Printf("[AIBudget] school lookup failed, usage is recorded without school_id: user_id=%d: %v", userID, err)
		}
		return nil
	}
	return row.SchoolID
}

type DailyCostRow struct {
	Date         string
	TotalCostUSD float64
	TotalTokens  int64
	CallCount    int64
}

type MonthlyCostRow struct {
	Month        string
	TotalCostUSD float64
	TotalTokens  int64
	CallCount    int64
}

type ModelCostRow struct {
	Model        string
	TotalCostUSD float64
	TotalTokens  int64
	CallCount    int64
	// RecordedPriceBasis は、期間内の行に記録した単価の根拠のうち、一番注意が必要なもの
	// （unknown > inherited > estimated > listed, #2004）。記録が1件も無ければ空文字。
	RecordedPriceBasis string
	// UnrecordedCalls は、トークン単価で計算したはずなのに根拠の記録が無い行（price_basis の列を足す前の行）の数。
	// 音声・ローカル推論・トークン0（Web 検索のツール料だけ）の行は、トークン単価を使っていないので数えない
	// （#2166 の Codex のレビュー）。数えると、トークン単価を使っていないモデルに今の単価表の根拠が付き、
	// 同じモデルの記録済みの根拠も、将来の単価表の変更で上書きされる。
	// provider の比較は、列の照合順序（utf8mb4 の _ci）で大文字・小文字を区別しない。
	UnrecordedCalls int64
}

// DailyCosts は過去 nDays 日間の日次集計を返す
func (r *APICallLogRepository) DailyCosts(nDays int) ([]DailyCostRow, error) {
	var rows []DailyCostRow
	since := time.Now().UTC().AddDate(0, 0, -nDays)
	err := r.db.Raw(`
		SELECT DATE(called_at) AS date,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY DATE(called_at)
		ORDER BY date ASC`, since).Scan(&rows).Error
	return rows, err
}

// MonthlyCosts は過去 nMonths ヶ月間の月次集計を返す
func (r *APICallLogRepository) MonthlyCosts(nMonths int) ([]MonthlyCostRow, error) {
	var rows []MonthlyCostRow
	since := time.Now().UTC().AddDate(0, -nMonths, 0)
	err := r.db.Raw(`
		SELECT DATE_FORMAT(called_at, '%Y-%m') AS month,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY month
		ORDER BY month ASC`, since).Scan(&rows).Error
	return rows, err
}

// BreakdownDimension は利用量の集計軸（#1294）。
//
// 列名を SQL へ直接埋めるため、外から来る文字列をそのまま使わず許可リストで固定する。
type BreakdownDimension string

const (
	BreakdownByFeature      BreakdownDimension = "feature"
	BreakdownByProvider     BreakdownDimension = "provider"
	BreakdownByModel        BreakdownDimension = "model"
	BreakdownByOrganization BreakdownDimension = "organization"
)

// breakdownColumns は集計軸と実際の列名の対応。ここに無い軸は受け付けない。
var breakdownColumns = map[BreakdownDimension]string{
	BreakdownByFeature:      "feature",
	BreakdownByProvider:     "provider",
	BreakdownByModel:        "model",
	BreakdownByOrganization: "organization_id",
}

// UsageBreakdownRow は軸ごとの集計結果。
type UsageBreakdownRow struct {
	Key           string  `json:"key"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	TotalTokens   int64   `json:"total_tokens"`
	CallCount     int64   `json:"call_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	CacheHitCount int64   `json:"cache_hit_count"`
}

// IsValidBreakdownDimension は集計軸が許可リストにあるかを返す。
func IsValidBreakdownDimension(dim BreakdownDimension) bool {
	_, ok := breakdownColumns[dim]
	return ok
}

// UsageBreakdown は指定軸で利用量を集計する（#1294）。
//
// 未設定の値（organization_id の NULL、provider の空文字）は 'unassigned' へ寄せる。
// 落とすと「合計は合うのに内訳が合わない」表になり、配賦漏れに気づけない。
func (r *APICallLogRepository) UsageBreakdown(ctx context.Context, since time.Time, dim BreakdownDimension) ([]UsageBreakdownRow, error) {
	col, ok := breakdownColumns[dim]
	if !ok {
		return nil, fmt.Errorf("unsupported breakdown dimension: %q", dim)
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(CAST(%s AS CHAR), ''), 'unassigned') AS `+"`key`"+`,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count,
		       AVG(latency_ms) AS avg_latency_ms,
		       SUM(cache_hit) AS cache_hit_count
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY %s
		ORDER BY total_cost_usd DESC`, col, col)

	var rows []UsageBreakdownRow
	err := r.db.WithContext(ctx).Raw(query, since).Scan(&rows).Error
	return rows, err
}

// ModelBreakdown はモデル別コスト内訳を返す
func (r *APICallLogRepository) ModelBreakdown(since time.Time) ([]ModelCostRow, error) {
	var rows []ModelCostRow
	err := r.db.Raw(`
		SELECT model,
		       SUM(cost_usd) AS total_cost_usd,
		       SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS call_count,
		       CASE MAX(CASE price_basis
		                    WHEN 'unknown' THEN 3
		                    WHEN 'inherited' THEN 2
		                    WHEN 'estimated' THEN 1
		                    WHEN 'listed' THEN 0
		                    ELSE -1 END)
		            WHEN 3 THEN 'unknown'
		            WHEN 2 THEN 'inherited'
		            WHEN 1 THEN 'estimated'
		            WHEN 0 THEN 'listed'
		            ELSE '' END AS recorded_price_basis,
		       SUM(CASE WHEN price_basis = ''
		                 AND prompt_tokens + completion_tokens > 0
		                 AND audio_seconds = 0 AND characters = 0
		                 AND provider IN ('', 'openai')
		                THEN 1 ELSE 0 END) AS unrecorded_calls
		FROM api_call_logs
		WHERE called_at >= ?
		GROUP BY model
		ORDER BY total_cost_usd DESC`, since).Scan(&rows).Error
	return rows, err
}

// TotalFallbackCostSince は指定日時以降の「OpenAIフォールバック分だけ」の合計コストを返す（#1293）。
//
// フォールバックの USD 上限はこの値で判定する。全コール合計で判定すると、
// 企業検索など通常の OpenAI 利用だけで上限に達し、ローカル障害時に
// フォールバックが一度も発動しなくなる（実データで月 $57 に対し既定上限 $20）。
func (r *APICallLogRepository) TotalFallbackCostSince(ctx context.Context, since time.Time) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).Model(&models.APICallLog{}).
		Where("via_fallback = ? AND called_at >= ?", true, since).
		Select("COALESCE(SUM(cost_usd), 0)").
		Scan(&total).Error
	return total, err
}

// TotalCostSince は指定日時以降の合計コストを返す
func (r *APICallLogRepository) TotalCostSince(since time.Time) (float64, error) {
	var total float64
	err := r.db.Model(&models.APICallLog{}).
		Where("called_at >= ?", since).
		Select("COALESCE(SUM(cost_usd), 0)").
		Scan(&total).Error
	return total, err
}

// CountSearchCallsSince は指定日時以降の Web Search 系モデル呼び出し回数を返す。
// company Search 予算（#587）のカウントに使う。model に "search" を含む行を対象とする。
func (r *APICallLogRepository) CountSearchCallsSince(since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.APICallLog{}).
		Where("called_at >= ? AND LOWER(model) LIKE ?", since, "%search%").
		Count(&count).Error
	return count, err
}

const apiCallLogRetentionLock = "api_call_log_retention"

// 日次合計へ畳む列。利用者 ID と遅延は残さない（SOCAIAGENT-528）。
// school_id / organization_id の NULL は 0 に寄せ、UNIQUE が NULL を別物として増殖しないようにする。
const apiCallLogRollupInsert = `
INSERT INTO api_call_log_daily (
  called_on, model, feature, provider, school_id, organization_id,
  prompt_stage, prompt_version, prompt_ab_group, price_basis, via_fallback,
  prompt_tokens, completion_tokens, total_tokens, cost_usd,
  audio_seconds, characters, web_search_calls, call_count
)
SELECT DATE(called_at), model, feature, provider,
       IFNULL(school_id, 0), IFNULL(organization_id, 0),
       prompt_stage, prompt_version, prompt_ab_group, price_basis, via_fallback,
       SUM(prompt_tokens), SUM(completion_tokens), SUM(total_tokens), SUM(cost_usd),
       SUM(audio_seconds), SUM(characters), SUM(web_search_calls), COUNT(*)
FROM api_call_logs
WHERE id IN (%s)
GROUP BY DATE(called_at), model, feature, provider,
         IFNULL(school_id, 0), IFNULL(organization_id, 0),
         prompt_stage, prompt_version, prompt_ab_group, price_basis, via_fallback
ON DUPLICATE KEY UPDATE
  prompt_tokens = prompt_tokens + VALUES(prompt_tokens),
  completion_tokens = completion_tokens + VALUES(completion_tokens),
  total_tokens = total_tokens + VALUES(total_tokens),
  cost_usd = cost_usd + VALUES(cost_usd),
  audio_seconds = audio_seconds + VALUES(audio_seconds),
  characters = characters + VALUES(characters),
  web_search_calls = web_search_calls + VALUES(web_search_calls),
  call_count = call_count + VALUES(call_count)`

const apiCallLogRetentionSelect = `
SELECT id FROM api_call_logs
WHERE called_at < ?
ORDER BY id
LIMIT ?
FOR UPDATE`

// RollupBatch は cutoff より古い行を最大 limit 件、日次合計へ足してから消す。
//
// 合計の加算と削除は同じトランザクションで行う。途中で落ちても、足した分が消されずに残らない。
// 複数台が同時に走ると二重に足すので、GET_LOCK を取れなかった回は何もしない（ran=false）。
func (r *APICallLogRepository) RollupBatch(ctx context.Context, cutoff time.Time, limit int) (int, bool, error) {
	if limit <= 0 {
		return 0, true, nil
	}
	var deleted int
	var ran bool
	err := r.db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		var got *int
		if err := conn.Raw("SELECT GET_LOCK(?, 0)", apiCallLogRetentionLock).Scan(&got).Error; err != nil {
			return err
		}
		if got == nil || *got != 1 {
			return nil
		}
		ran = true
		defer conn.Exec("SELECT RELEASE_LOCK(?)", apiCallLogRetentionLock)
		return conn.Transaction(func(tx *gorm.DB) error {
			n, err := rollupLockedBatch(tx, cutoff, limit)
			deleted = n
			return err
		})
	})
	return deleted, ran, err
}

func rollupLockedBatch(tx *gorm.DB, cutoff time.Time, limit int) (int, error) {
	var ids []uint
	if err := tx.Raw(apiCallLogRetentionSelect, cutoff, limit).Scan(&ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	insert := fmt.Sprintf(apiCallLogRollupInsert, placeholders(len(ids)))
	if err := tx.Exec(insert, args...).Error; err != nil {
		return 0, err
	}
	del := "DELETE FROM api_call_logs WHERE id IN (" + placeholders(len(ids)) + ")"
	if err := tx.Exec(del, args...).Error; err != nil {
		return 0, err
	}
	return len(ids), nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
