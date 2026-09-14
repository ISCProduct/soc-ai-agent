package diagnosis

import (
	"encoding/json"
	"strings"
)

// MinConfidenceForFinal は recommendations を「確定」扱いできる最低信頼度。
const MinConfidenceForFinal = 55

// ForcesProvisional は品質レポートの内容から暫定表示を強制すべきか判定する。
func ForcesProvisional(confidence int, flags []string) bool {
	if confidence > 0 && confidence < MinConfidenceForFinal {
		return true
	}
	for _, f := range flags {
		switch strings.TrimSpace(f) {
		case "no_chat_evidence", "thin_chat_evidence", "mostly_choice_only",
			"choice_reason_contradiction", "thin_match_evidence",
			"few_measured_axes", "no_measured_axes", "saturated_matches":
			return true
		}
	}
	return false
}

// ParseFlagsJSON は DB に保存した flags JSON を読む。
func ParseFlagsJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var flags []string
	if err := json.Unmarshal([]byte(raw), &flags); err != nil {
		return nil
	}
	return flags
}
