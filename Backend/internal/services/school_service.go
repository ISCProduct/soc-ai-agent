package services

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

// ResolveAdminAccess は管理者の担当校を返す。
// 0件(未割当)の管理者は「システム管理者」として学校による絞り込みを受けない(restricted=false)。
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

// ListAccessibleSchools は管理者がフィルタUIで選べる学校一覧を返す。
// 担当校がある管理者にはその学校群を、無制限(未割当)管理者には全学校を返す。
func (s *SchoolService) ListAccessibleSchools(userID uint) (restricted bool, schools []models.School, err error) {
	assigned, err := s.repo.ListSchoolsForAdmin(userID)
	if err != nil {
		return false, nil, err
	}
	if len(assigned) > 0 {
		return true, assigned, nil
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
