package models

import "time"

// TrainingConsentTextVersion は、学習利用の同意の画面（frontend/components/profile/TrainingConsentCard.tsx）に
// 出している文言の版（#2168）。同意の履歴に記録し、「どの文言に同意したか」を後から確かめられるようにする。
//
// TrainingConsentCard.tsx の同意の文言（説明文・箇条書き・スイッチの文言）を変えたら、この版を上げること。
// 版を上げずに文言だけ変えると、変更前に同意した人も新しい文言に同意したように記録される。
const TrainingConsentTextVersion = "2026-11-11"

// TrainingConsentLegacyTextVersion は、履歴の記録を始める前（000075 の適用前）から同意していた人の行の版。
// その人がいつ・どの文言に同意したかは記録が無く分からないため、実在の版とは別の値にして区別する。
// 行は 000075 の up で作り、created_at はマイグレーションの適用時刻（同意した日時ではない）。
// migrations/000075_create_training_consent_events.up.sql の値と同じにすること（テストで確かめる）。
const TrainingConsentLegacyTextVersion = "legacy-before-history"

// TrainingConsentEvent は、学習利用の同意を変更した操作の履歴（#2168）。
// users.allow_training_use の更新と同じトランザクションで1行追加し、更新はしない。
// 追加は UserPreferenceRepository.SetTrainingUse で、created_at を DB の時計（NOW(3)）で入れる。
// スキーマは migrations/000075_create_training_consent_events.up.sql で管理。
type TrainingConsentEvent struct {
	ID                 uint      `gorm:"primaryKey"`
	UserID             uint      `gorm:"not null"`
	AllowTrainingUse   bool      `gorm:"not null"`
	ConsentTextVersion string    `gorm:"size:32;not null"`
	CreatedAt          time.Time `gorm:"not null"`
}

func (TrainingConsentEvent) TableName() string { return "training_consent_events" }
