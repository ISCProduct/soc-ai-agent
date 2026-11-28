package companyfetch

import (
	"os"
	"strings"
)

// BotToken は robots.txt の User-agent 行で照合する製品名。
const BotToken = "SocAI"

// BotContact は自動取得の連絡先(URL かメールアドレス)を返す。
// 公開リポジトリに載せないため COMPANY_BOT_CONTACT で指定する。
func BotContact() string {
	return strings.TrimSpace(os.Getenv("COMPANY_BOT_CONTACT"))
}

// UserAgent は外部サイトを読むときの User-Agent を返す。
// 連絡先が未設定なら載せない(仮のURLで他人のドメインを名乗らない)。
func UserAgent() string {
	if c := BotContact(); c != "" {
		return "Mozilla/5.0 (compatible; " + BotToken + "/1.0; +" + c + ")"
	}
	return "Mozilla/5.0 (compatible; " + BotToken + "/1.0)"
}
