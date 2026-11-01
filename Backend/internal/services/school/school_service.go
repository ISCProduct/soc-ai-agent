package school

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrSchoolNotFound = errors.New("school not found")
	// 担当校は学園をまたげない（#1675）。
	ErrMemberOrganizationMismatch = errors.New("member belongs to another organization")
	ErrMemberNotFound             = errors.New("member not found")
	// ErrUserReaderNotConfigured は学園一致チェック用のユーザー参照が注入されていない状態。
	// errors.Is で判定できるようにして、テストが「別の理由のエラー」で緑にならないようにする。
	ErrUserReaderNotConfigured = errors.New("school service is not configured with a user reader")
	ErrSchoolNameRequired      = errors.New("name is required")
	ErrSchoolOrgRequired       = errors.New("organization_id is required")
	ErrSchoolAlreadyAssigned   = errors.New("admin already assigned to this school")
	ErrCompanyAlreadyApproved  = errors.New("company already approved for this school")
)

// UserReader は担当校割当の学園一致チェックに使う最小のユーザー参照（#1675）。
type UserReader interface {
	GetUserByID(id uint) (*entity.User, error)
}

// SchoolService は学園(Organization)配下の個別校・担当管理者・企業掲載承認リストを管理する。
type SchoolService struct {
	repo  repository.SchoolRepository
	users UserReader
}

// NewSchoolService は学校サービスを作る。
//
// users は担当校割当（AddMember）の学園一致チェックに使う。AddMember を使わない呼び出し元は
// nil を明示的に渡す（nil のときは AddMember がエラーで止まる = fail-close）。
// 可変長にして省略できるようにすると渡し忘れをコンパイラが拾えないため、位置引数で必須にする。
// 検査を飛ばして割当が通る方が危険なので、未注入を黙って許さない。
func NewSchoolService(repo repository.SchoolRepository, users UserReader) *SchoolService {
	return &SchoolService{repo: repo, users: users}
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
	// 担当校は学園をまたげない。またげると、他学園の職員に担当校を割り当てるだけで
	// その学校の学生データへ到達させられる（#1675）。
	if s.users == nil {
		// 未注入のまま割当を通すと学園一致を検査できない。止める方を選ぶ。
		return ErrUserReaderNotConfigured
	}
	member, err := s.users.GetUserByID(userID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrMemberNotFound
	}
	if member.OrganizationID != school.OrganizationID {
		return ErrMemberOrganizationMismatch
	}
	if err := s.repo.AddMember(&models.AdminSchoolMembership{UserID: userID, SchoolID: schoolID}); err != nil {
		if isDuplicateEntryErr(err) {
			return ErrSchoolAlreadyAssigned
		}
		return err
	}
	return nil
}

// EffectiveSchoolID は、学校が本人の学園に属しているときだけ school_id を返す（#1675）。
//
// users.school_id はプロフィールの学校名（本人の自己申告）から解決される。解決側は
// organization_id で絞り、過去の越境行も migration 000050 で外したが、1件ごとの認可判定を
// データの綺麗さに依存させない。学園が一致しなければ nil =未所属として扱い、担当校ありの
// 管理者・職員には見せない（fail-close）。
func (s *SchoolService) EffectiveSchoolID(organizationID uint, schoolID *uint) (*uint, error) {
	if schoolID == nil {
		return nil, nil
	}
	school, err := s.repo.FindByID(*schoolID)
	if err != nil {
		return nil, err
	}
	if school == nil || school.OrganizationID != organizationID {
		return nil, nil
	}
	return schoolID, nil
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
