// Package userpreference は学生本人の希望条件（#1094）と、公開・学習利用の同意（#2167）を扱う。
package userpreference

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
)

type Service struct {
	repo *repositories.UserPreferenceRepository
}

func NewService(repo *repositories.UserPreferenceRepository) *Service {
	return &Service{repo: repo}
}

// Get は希望条件と企業への公開同意を返す。
// 未設定でも空の希望条件を返し、フロントの分岐を減らす。
func (s *Service) Get(userID uint) (*models.UserPreference, bool, error) {
	pref, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, false, err
	}
	if pref == nil {
		pref = &models.UserPreference{UserID: userID}
	}
	allow, err := s.repo.GetScoutVisibility(userID)
	if err != nil {
		return nil, false, err
	}
	return pref, allow, nil
}

// Save は希望条件を保存し、公開同意が指定されていれば更新して、保存後の公開同意を返す。
// allowScout が nil なら公開同意は変更しない。
func (s *Service) Save(pref *models.UserPreference, allowScout *bool) (bool, error) {
	if err := s.repo.Upsert(pref); err != nil {
		return false, err
	}
	if allowScout != nil {
		if err := s.repo.SetScoutVisibility(pref.UserID, *allowScout); err != nil {
			return false, err
		}
	}
	return s.repo.GetScoutVisibility(pref.UserID)
}

// TrainingConsent は学習利用の同意の現在値と、最後に変更した日時を返す（#2168）。
func (s *Service) TrainingConsent(userID uint) (repositories.TrainingConsentState, error) {
	return s.repo.GetTrainingConsent(userID)
}

// SetTrainingUse は学習利用の同意を更新し、同意の履歴を同じトランザクションで残す（#2168）。
// textVersion が今の文言の版と一致することは呼び出し側で確かめる。
func (s *Service) SetTrainingUse(userID uint, allow bool, textVersion string) error {
	return s.repo.SetTrainingUse(userID, allow, textVersion)
}
