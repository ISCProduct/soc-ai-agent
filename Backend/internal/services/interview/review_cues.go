package interview

import (
	"Backend/internal/models"
	"time"
)

// ReviewCue は発話を録画の再生位置へ合わせる目安。
// OffsetSeconds が nil のときは、面接開始時刻が無く位置を出せない。
type ReviewCue struct {
	UtteranceID   uint   `json:"utterance_id"`
	Role          string `json:"role"`
	OffsetSeconds *int   `json:"offset_seconds"`
}

// ReviewCues は発話時刻と面接開始時刻の差を秒で返す。
// 録画は面接開始APIの直後に始まるため、これはフレーム単位の時刻ではなく目安。
// 開始より前の時刻は 0 秒に揃える。
func ReviewCues(started *time.Time, utterances []models.InterviewUtterance) []ReviewCue {
	cues := make([]ReviewCue, 0, len(utterances))
	for _, utterance := range utterances {
		cue := ReviewCue{UtteranceID: utterance.ID, Role: utterance.Role}
		if started != nil {
			seconds := int(utterance.CreatedAt.Sub(*started).Round(time.Second) / time.Second)
			if seconds < 0 {
				seconds = 0
			}
			cue.OffsetSeconds = &seconds
		}
		cues = append(cues, cue)
	}
	return cues
}
