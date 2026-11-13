package shared

// CompanyAccessReader は、ユーザーが管理者か、企業の所有者かを返す。
// 実装は repositories.CompanyAccessRepository。
type CompanyAccessReader interface {
	IsAdmin(userID uint) (bool, error)
	OwnsCompany(userID, companyID uint) (bool, error)
}

// UserIsAdmin は users.is_admin を返す。r が nil なら false（fail-closed）。
func UserIsAdmin(r CompanyAccessReader, userID uint) (bool, error) {
	if r == nil {
		return false, nil
	}
	return r.IsAdmin(userID)
}

// UserOwnsCompany は company_ownerships に (user_id, company_id) があるか返す。
// r が nil、または companyID が 0 なら false（fail-closed）。
func UserOwnsCompany(r CompanyAccessReader, userID, companyID uint) (bool, error) {
	if r == nil || companyID == 0 {
		return false, nil
	}
	return r.OwnsCompany(userID, companyID)
}
