package migrations

import (
	"strings"
	"testing"

	"Backend/internal/models"
)

// #2262 のレビュー: 000075 の up は、履歴の記録を始める前から同意していた人に履歴を1行ずつ作る。
// その行の版は models.TrainingConsentLegacyTextVersion と同じ値で、同意済み（allow_training_use = 1）の人だけが対象。
// SQL とコードの定数がずれると、「記録の前からの同意」の行をアプリ側で見分けられなくなる。
// （実際の MySQL での up→down→up と、行ができることは PR で確かめている。ここではずれだけを見る。）
func TestTrainingConsentBackfill_UsesLegacyVersion(t *testing.T) {
	data, err := migrationFiles.ReadFile("000075_create_training_consent_events.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	// コメントを除いた SQL 本体で確かめる（コメントの中の文字列で通らないように）
	var body []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		body = append(body, line)
	}
	sql := strings.Join(body, "\n")

	for _, want := range []string{
		"INSERT INTO `training_consent_events`",
		"SELECT `id`, 1, '" + models.TrainingConsentLegacyTextVersion + "', NOW(3)",
		"WHERE `allow_training_use` = 1",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("000075 の up に %q が無い", want)
		}
	}
	if models.TrainingConsentLegacyTextVersion == models.TrainingConsentTextVersion {
		t.Error("記録の前からの同意の版は、実在の文言の版と別の値にする")
	}
	if len(models.TrainingConsentLegacyTextVersion) > 32 {
		t.Errorf("consent_text_version（varchar(32)）に入らない: %q", models.TrainingConsentLegacyTextVersion)
	}
}
