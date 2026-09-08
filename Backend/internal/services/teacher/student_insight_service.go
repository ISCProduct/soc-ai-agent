package teacher

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/repositories"
)

// StudentLister は生徒一覧の取得面。テストで差し替えられるよう最小の面に絞る。
type StudentLister interface {
	ListStudentsPaged(limit, offset int, query string, schoolID *uint) ([]entity.User, int64, error)
}

// ScoreBatchReader は複数生徒のスコアを一括で読む面。
type ScoreBatchReader interface {
	FindLatestScoresByUsers(userIDs []uint) (map[uint]map[string]float64, error)
}

// IndustryReader は業界マスタの読み出し面。
// 既存の IndustryRepository.ListActive に合わせて IndustryOption を受ける。
type IndustryReader interface {
	ListActive() ([]repositories.IndustryOption, error)
}

// IndustryProfileReader は業界プロファイルの読み出し面。
type IndustryProfileReader interface {
	ListAll() ([]models.IndustryWeightProfile, error)
}

type StudentInsightService struct {
	users      StudentLister
	scores     ScoreBatchReader
	industries IndustryReader
	profiles   IndustryProfileReader
}

func NewStudentInsightService(
	users StudentLister,
	scores ScoreBatchReader,
	industries IndustryReader,
	profiles IndustryProfileReader,
) *StudentInsightService {
	return &StudentInsightService{users: users, scores: scores, industries: industries, profiles: profiles}
}

// TendencyResult は一覧APIのレスポンス。
type TendencyResult struct {
	Students []StudentTendency `json:"students"`
	Total    int64             `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

// ListTendencies は担当生徒の傾向タイプと向いている業界を返す（#1027）。
//
// schoolID は EchoAdminSchoolScope が解決した担当校。nil は絞り込みなし
// （担当校を持たないシステム管理者）。
//
// 生徒数に対して N+1 にしないため、スコアは FindLatestScoresByUsers で
// 一括取得し、業界と業界プロファイルはページ全体で1回ずつしか読まない。
func (s *StudentInsightService) ListTendencies(limit, offset int, query string, schoolID *uint) (*TendencyResult, error) {
	students, total, err := s.users.ListStudentsPaged(limit, offset, query, schoolID)
	if err != nil {
		return nil, err
	}

	result := &TendencyResult{
		Students: make([]StudentTendency, 0, len(students)),
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}
	// 担当生徒が0人でも空一覧を返す（PRD 境界値）。
	if len(students) == 0 {
		return result, nil
	}

	userIDs := make([]uint, 0, len(students))
	for _, u := range students {
		userIDs = append(userIDs, u.ID)
	}
	scoresByUser, err := s.scores.FindLatestScoresByUsers(userIDs)
	if err != nil {
		return nil, err
	}

	allIndustries, err := s.industries.ListActive()
	if err != nil {
		return nil, err
	}
	industries := topLevelIndustries(allIndustries)
	rawProfiles, err := s.profiles.ListAll()
	if err != nil {
		return nil, err
	}
	profileByIndustry := make(map[uint]*models.IndustryWeightProfile, len(rawProfiles))
	for i := range rawProfiles {
		profileByIndustry[rawProfiles[i].IndustryID] = &rawProfiles[i]
	}

	for _, u := range students {
		result.Students = append(result.Students, BuildTendency(
			u.ID, u.Name, u.Email, scoresByUser[u.ID], industries, profileByIndustry,
		))
	}
	return result, nil
}

// topLevelIndustries は大分類(level 0)だけに絞る（#1027）。
//
// ListActive は親(level 0)と子(level 1)を両方返すため、そのまま順位付けすると
// TOP3 が「情報通信業 / ソフトウェア開発 / Webサービス」のように
// 1ファミリで埋まり、進路指導の情報量がほとんど無くなる。
//
// なお industries.parent_id はシードが設定しておらず全件 NULL のため、
// 親子関係は level でしか判別できない（別途 #929 系の是正対象）。
func topLevelIndustries(all []repositories.IndustryOption) []repositories.IndustryOption {
	top := make([]repositories.IndustryOption, 0, len(all))
	for _, i := range all {
		if i.Level == 0 {
			top = append(top, i)
		}
	}
	// level が一つも 0 でない構成なら、絞らずに全件を使う（表示が空になるより良い）。
	if len(top) == 0 {
		return all
	}
	return top
}
