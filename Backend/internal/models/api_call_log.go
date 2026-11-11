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
	// PromptStage / PromptVersion は工程別の品質比較に使う。
	PromptStage   string `gorm:"size:64;not null;default:'unknown'" json:"prompt_stage"`
	PromptVersion string `gorm:"size:64;not null;default:''" json:"prompt_version,omitempty"`
	// PromptABGroup は A/B で割り当てた版（#1976）。A/B の対象外の呼び出しは空。
	PromptABGroup string `gorm:"column:prompt_ab_group;size:64;not null;default:''" json:"prompt_ab_group,omitempty"`
	// 実行主体。バッチ経路は主体を持たないため nil。
	UserID *uint `json:"user_id,omitempty"`
	// 呼び出し時点の学校所属。users.school_id の後日変更で過去実績を移動させない。
	SchoolID *uint `json:"school_id,omitempty"`
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
	// web_search ツールの呼び出し回数。トークンとは別に1コール単位で課金される。
	// 単価ではなく生の回数を残すのは AudioSeconds / Characters と同じ理由で、
	// 価格改定が入っても後から再計算できるようにするため。
	// これが無いと、同じ gpt-4o-mini でも通常のチャットと web_search を
	// 区別できず、検索コストの85%を占めるツール料が記録から抜ける。
	WebSearchCalls int `gorm:"not null;default:0" json:"web_search_calls"`
	// PriceBasis は、記録した時点のトークン単価の根拠（listed / estimated / inherited / unknown, #2004）。
	// 集計時に今の単価表から判定すると、単価表を上書きしたときに過去の行まで表示が変わるため、記録時に残す。
	// 空文字は記録なし（音声の単価で計算した行・ローカル推論の行・この列を足す前の行）。migrations/000062。
	PriceBasis string `gorm:"size:16;not null;default:''" json:"price_basis,omitempty"`
	// 複合インデックス (called_at, provider) と (organization_id, called_at) は
	// migrations/000031 で定義する。AutoMigrate は使わない方針のためタグには書かない。
}
