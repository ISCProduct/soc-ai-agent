package discord

// Discord Interactions API の型定義（HTTP Interactionsで必要な最小限のみ）。
// https://discord.com/developers/docs/interactions/receiving-and-responding

type InteractionType int

const (
	InteractionTypePing               InteractionType = 1
	InteractionTypeApplicationCommand InteractionType = 2
	InteractionTypeModalSubmit        InteractionType = 5
)

type ResponseType int

const (
	ResponseTypePong                             ResponseType = 1
	ResponseTypeChannelMessageWithSource         ResponseType = 4
	ResponseTypeDeferredChannelMessageWithSource ResponseType = 5
	ResponseTypeModal                            ResponseType = 9
)

type ComponentType int

const (
	ComponentTypeActionRow ComponentType = 1
	ComponentTypeTextInput ComponentType = 4
)

type TextInputStyle int

const (
	TextInputStyleShort TextInputStyle = 1
)

// Interaction はDiscordから届くリクエストボディ。
// ApplicationID/Token は、type=5(deferred)で即時応答した後にフォローアップメッセージを
// 送る(PATCH /webhooks/{application_id}/{token}/messages/@original)際に必要。
type Interaction struct {
	Type          InteractionType  `json:"type"`
	ApplicationID string           `json:"application_id"`
	Token         string           `json:"token"`
	Member        *Member          `json:"member"`
	Data          *InteractionData `json:"data"`
}

type Member struct {
	Roles []string `json:"roles"`
	User  *User    `json:"user"`
}

// User は誰が本番を止めたかを記録するために使う。
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// ActorLabel は監査ログ用の実行者表記。取得できなければ "unknown"。
func (i *Interaction) ActorLabel() string {
	if i == nil || i.Member == nil || i.Member.User == nil {
		return "unknown"
	}
	if i.Member.User.Username != "" {
		return i.Member.User.Username + "(" + i.Member.User.ID + ")"
	}
	return i.Member.User.ID
}

// InteractionData はスラッシュコマンド名、またはモーダル送信時の custom_id・入力値を含む。
type InteractionData struct {
	Name       string          `json:"name,omitempty"`
	CustomID   string          `json:"custom_id,omitempty"`
	Components []Component     `json:"components,omitempty"`
	Options    []CommandOption `json:"options,omitempty"`
}

// CommandOption はスラッシュコマンドの引数（/prod state:on の "state"）。
// Value を any で受けるのは、文字列以外の型の引数を持つコマンドが将来増えても
// ペイロード全体のデコードが失敗しないようにするため。
type CommandOption struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type Component struct {
	Type        ComponentType  `json:"type"`
	CustomID    string         `json:"custom_id,omitempty"`
	Style       TextInputStyle `json:"style,omitempty"`
	Label       string         `json:"label,omitempty"`
	Value       string         `json:"value,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	Components  []Component    `json:"components,omitempty"`
}

// InteractionResponse はDiscordへ返すレスポンス。
type InteractionResponse struct {
	Type ResponseType             `json:"type"`
	Data *InteractionResponseData `json:"data,omitempty"`
}

type InteractionResponseData struct {
	Content    string      `json:"content,omitempty"`
	Flags      int         `json:"flags,omitempty"`
	CustomID   string      `json:"custom_id,omitempty"`
	Title      string      `json:"title,omitempty"`
	Components []Component `json:"components,omitempty"`
}

// EphemeralFlag は本人にしか見えないメッセージを表すフラグ値。
const EphemeralFlag = 1 << 6

// ModalCustomIDProdUptime / TextInputCustomIDDate はコマンド・モーダル間の識別子。
const (
	CommandNameProdUptime     = "prod-uptime"
	CommandNameProdUptimeList = "prod-uptime-list"
	CommandNameProd           = "prod"
	CommandNameStaging        = "staging"
	OptionNameState           = "state"
	ModalCustomIDProdUptime   = "prod_uptime_modal"
	TextInputCustomIDDate     = "prod_uptime_date"
)

// FindOptionString はスラッシュコマンドの文字列引数を取り出す。
// 見つからない、または文字列でない場合は空文字を返す。
func FindOptionString(options []CommandOption, name string) string {
	for _, o := range options {
		if o.Name != name {
			continue
		}
		if v, ok := o.Value.(string); ok {
			return v
		}
		return ""
	}
	return ""
}

// FindComponentValue はモーダル送信データのネストしたComponentsから指定custom_idの入力値を探す。
func FindComponentValue(components []Component, customID string) string {
	for _, row := range components {
		for _, c := range row.Components {
			if c.CustomID == customID {
				return c.Value
			}
		}
		if row.CustomID == customID {
			return row.Value
		}
	}
	return ""
}
