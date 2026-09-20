package models

import "time"

// APICallLog OpenAI APIコール記録
type APICallLog struct {
	ID               uint    `gorm:"primaryKey"           json:"id"`
	Model            string  `gorm:"size:100;not null"    json:"model"`
	PromptTokens     int     `gorm:"not null;default:0"   json:"prompt_tokens"`
	CompletionTokens int     `gorm:"not null;default:0"   json:"completion_tokens"`
	TotalTokens      int     `gorm:"not null;default:0"   json:"total_tokens"`
	CostUSD          float64 `gorm:"default:0" json:"cost_usd"`
	// Provider / ViaFallback は課金対象の切り分けに使う（#1293）。
	// api_call_logs は推論先に関係なく記録されるため、これが無いと
	// ローカル推論（無料）や通常の OpenAI 利用とフォールバック分を区別できない。
	Provider    string    `gorm:"size:32;not null;default:''" json:"provider"`
	ViaFallback bool      `gorm:"not null;default:0;index:idx_api_call_logs_fallback_called_at,priority:1" json:"via_fallback"`
	CalledAt    time.Time `gorm:"not null;index;index:idx_api_call_logs_fallback_called_at,priority:2" json:"called_at"`

	// 配賦軸（#1294）。どの機能が・誰の操作で・どこに配賦されるコストかを記録する。
	// これが無いと「ローカルAI化でどの機能の費用が下がったか」を示せない。
	//
	// Feature は呼び出し元が定数で渡す。渡し漏れた経路は 'unknown' のまま記録され、
	// その件数が「まだ計測できていない経路」の指標になる（DesignDoc §8）。
	Feature string `gorm:"size:64;not null;default:'unknown'" json:"feature"`
	// 実行主体。バッチ経路は主体を持たないため nil。
	UserID *uint `json:"user_id,omitempty"`
	// 配賦先の学校/企業。解決できない場合は nil。
	OrganizationID *uint `json:"organization_id,omitempty"`
	// STT/TTS はトークンではなく音声の長さが課金単位になる。
	AudioSeconds float64 `gorm:"type:decimal(10,2);not null;default:0" json:"audio_seconds"`
	// TTS は文字数が課金単位。トークンでもなく秒でもないため独立させる。
	Characters int `gorm:"not null;default:0" json:"characters"`
	// ローカル化が体験に与える影響を見るため。
	LatencyMs int `gorm:"not null;default:0" json:"latency_ms"`
	// キャッシュで外部呼び出しを回避した件数を数えるため。
	CacheHit bool `gorm:"not null;default:0" json:"cache_hit"`
	// 複合インデックス (called_at, provider) と (organization_id, called_at) は
	// migrations/000031 で定義する。AutoMigrate は使わない方針のためタグには書かない。
}
