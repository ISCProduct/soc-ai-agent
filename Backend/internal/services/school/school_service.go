package school

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrSchoolNotFound         = errors.New("school not found")
	ErrSchoolNameRequired     = errors.New("name is required")
	ErrSchoolOrgRequired      = errors.New("organization_id is required")
	ErrSchoolAlreadyAssigned  = errors.New("admin already assigned to this school")
	ErrCompanyAlreadyApproved = errors.New("company already approved for this school")
)

// SchoolService は学園(Organization)配下の個別校・担当管理者・企業掲載承認リストを管理する。
type SchoolService struct {
	repo repository.SchoolRepository
}

func NewSchoolService(repo repository.SchoolRepository) *SchoolService {
	return &SchoolService{repo: repo}
}

type CreateSchoolInput struct {
	OrganizationID uint
	Name           string
}

func (s *SchoolService) Create(input CreateSchoolInput) (*models.School, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrSchoolNameRequired
	}
	if input.OrganizationID == 0 {
		return nil, ErrSchoolOrgRequired
	}
	school := &models.School{
		OrganizationID: input.OrganizationID,
		Name:           name,
		Status:         models.SchoolStatusActive,
	}
	if err := s.repo.Create(school); err != nil {
		return nil, err
	}
	return school, nil
}

func (s *SchoolService) Get(id uint) (*models.School, error) {
	school, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if school == nil {
		return nil, ErrSchoolNotFound
	}
	return school, nil
}

func (s *SchoolService) List(limit, offset int) ([]models.School, int64, error) {
	return s.repo.List(limit, offset)
}

// AddMember は管理者(先生)を学校の担当として割り当てる。
func (s *SchoolService) AddMember(userID, schoolID uint) error {
	school, err := s.repo.FindByID(schoolID)
	if err != nil {
		return err
	}
	if school == nil {
		return ErrSchoolNotFound
	}
	if err := s.repo.AddMember(&models.AdminSchoolMembership{UserID: userID, SchoolID: schoolID}); err != nil {
		if isDuplicateEntryErr(err) {
			return ErrSchoolAlreadyAssigned
		}
		return err
	}
	return nil
}

func (s *SchoolService) RemoveMember(userID, schoolID uint) error {
	return s.repo.RemoveMember(userID, schoolID)
}

// ResolveAdminAccess はユーザーの担当校一覧（membership）だけを返す。is_admin は見ない。
// 対象ユーザーの担当状況を調べる用途（メンバー管理の相手側確認など）に使う。
// リクエスト主体のスコープ判定には ResolveAccess を使うこと（is_admin を考慮する）。
func (s *SchoolService) ResolveAdminAccess(userID uint) (restricted bool, schoolIDs []uint, err error) {
	schools, err := s.repo.ListSchoolsForAdmin(userID)
	if err != nil {
		return false, nil, err
	}
	if len(schools) == 0 {
		return false, nil, nil
	}
	ids := make([]uint, len(schools))
	for i, sc := range schools {
		ids[i] = sc.ID
	}
	return true, ids, nil
}

// ResolveAccess はリクエスト主体の実効スコープを返す。
//
// 「無制限（restricted=false, 全校閲覧）」は is_admin のときだけ成立する。
// 職員（is_admin=false）は担当校0件でも restricted=true（＝allowedが空＝何も見えない）。
// これを守らないと、担当校未設定の職員が全校の学生PIIを閲覧できてしまう。
func (s *SchoolService) ResolveAccess(isAdmin bool, userID uint) (restricted bool, schoolIDs []uint, err error) {
	schools, err := s.repo.ListSchoolsForAdmin(userID)
	if err != nil {
		return false, nil, err
	}
	if isAdmin && len(schools) == 0 {
		return false, nil, nil // 無制限のプラットフォーム管理者
	}
	ids := make([]uint, len(schools))
	for i, sc := range schools {
		ids[i] = sc.ID
	}
	return true, ids, nil
}

// CanAdminAccessSchool は、対象リソースの学校ID(未割当ならnil)に対してリクエスト主体が
// アクセスしてよいかを判定する。無制限admin(is_admin かつ担当校未割当)は常にtrue。
// それ以外（担当校ありの制限admin・職員）は、targetSchoolIDがnilの場合および担当校一覧に
// 含まれない場合はfalseを返す(fail-closed)。パスパラメータで単一リソースを直接指定する
// エンドポイントで、対象リソースをロードした後に呼び出すことを想定する(#980/#981/#982/#984)。
func (s *SchoolService) CanAdminAccessSchool(isAdmin bool, adminUserID uint, targetSchoolID *uint) (bool, error) {
	restricted, allowedSchoolIDs, err := s.ResolveAccess(isAdmin, adminUserID)
	if err != nil {
		return false, err
	}
	if !restricted {
		return true, nil
	}
	if targetSchoolID == nil {
		return false, nil
	}
	for _, id := range allowedSchoolIDs {
		if id == *targetSchoolID {
			return true, nil
		}
	}
	return false, nil
}

// ListAccessibleSchools は主体がフィルタUIで選べる学校一覧を返す。
// 担当校がある主体にはその学校群を、無制限(is_admin かつ未割当)管理者には全学校を返す。
// 職員で担当校0件なら空（何も選べない）。
func (s *SchoolService) ListAccessibleSchools(isAdmin bool, userID uint) (restricted bool, schools []models.School, err error) {
	assigned, err := s.repo.ListSchoolsForAdmin(userID)
	if err != nil {
		return false, nil, err
	}
	if len(assigned) > 0 {
		return true, assigned, nil
	}
	if !isAdmin {
		return true, []models.School{}, nil // 職員で担当校なし＝何も見えない
	}
	all, _, err := s.repo.List(1000, 0)
	if err != nil {
		return false, nil, err
	}
	return false, all, nil
}

func (s *SchoolService) AddCompanyApproval(schoolID, companyID uint) error {
	school, err := s.repo.FindByID(schoolID)
	if err != nil {
		return err
	}
	if school == nil {
		return ErrSchoolNotFound
	}
	if err := s.repo.AddCompanyApproval(&models.SchoolCompanyApproval{SchoolID: schoolID, CompanyID: companyID}); err != nil {
		if isDuplicateEntryErr(err) {
			return ErrCompanyAlreadyApproved
		}
		return err
	}
	return nil
}

func (s *SchoolService) RemoveCompanyApproval(schoolID, companyID uint) error {
	return s.repo.RemoveCompanyApproval(schoolID, companyID)
}

func (s *SchoolService) ListApprovedCompanyIDs(schoolID uint) ([]uint, error) {
	return s.repo.ListApprovedCompanyIDs(schoolID)
}

// isDuplicateEntryErr はMySQLの一意制約違反(Error 1062)かどうかを判定する。
func isDuplicateEntryErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
