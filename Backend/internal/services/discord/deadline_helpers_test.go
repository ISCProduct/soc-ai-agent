package discord

import "time"

// todayJSTPlus は JST の今日から n 日後（YYYY-MM-DD）。
func todayJSTPlus(n int) string {
	return time.Now().In(jst()).AddDate(0, 0, n).Format(dateLayout)
}
