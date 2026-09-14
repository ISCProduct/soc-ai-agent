package repositories

import "strings"

// IsInterviewSnapshotSession は面接レポート用の分離セッションか。
// チャット診断・マッチングの session_id とは別系統。
func IsInterviewSnapshotSession(sessionID string) bool {
	return strings.HasPrefix(sessionID, "interview-")
}
