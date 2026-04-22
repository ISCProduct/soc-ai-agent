package services

// 管理者判定ロジックのセキュリティテスト
// 実行: cd Backend && go test ./internal/services/... -run TestAdmin -v

import (
	"Backend/domain/entity"
	"errors"
	"testing"
)

// --- isAdminIdentity テスト ---

// TestIsAdminIdentity_EmailMatch はメールアドレスが ADMIN_EMAILS に含まれるとき true を返すことを検証する
func TestIsAdminIdentity_EmailMatch(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")

	if !isAdminIdentity("admin@example.com") {
		t.Error("ADMIN_EMAILS に含まれるメールアドレスは管理者と判定されるべき")
	}
}

// TestIsAdminIdentity_EmailMatchCaseInsensitive はメールアドレス照合が大文字小文字を区別しないことを検証する
func TestIsAdminIdentity_EmailMatchCaseInsensitive(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")

	if !isAdminIdentity("ADMIN@EXAMPLE.COM") {
		t.Error("メールアドレス照合は大文字小文字を区別しないべき")
	}
}

// TestIsAdminIdentity_NoMatch はメールアドレスが ADMIN_EMAILS に含まれないとき false を返すことを検証する
func TestIsAdminIdentity_NoMatch(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")

	if isAdminIdentity("user@example.com") {
		t.Error("ADMIN_EMAILS に含まれないメールアドレスは管理者と判定されるべきではない")
	}
}

// TestIsAdminIdentity_EmptyAdminEmails は ADMIN_EMAILS 未設定のとき常に false を返すことを検証する
func TestIsAdminIdentity_EmptyAdminEmails(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "")

	if isAdminIdentity("admin@example.com") {
		t.Error("ADMIN_EMAILS 未設定では誰も管理者と判定されるべきではない")
	}
}

// TestIsAdminIdentity_MultipleEmails は複数のメールアドレスが設定されているとき正しく照合することを検証する
func TestIsAdminIdentity_MultipleEmails(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin1@example.com, admin2@example.com, admin3@example.com")

	if !isAdminIdentity("admin2@example.com") {
		t.Error("複数設定のうち一致するメールアドレスは管理者と判定されるべき")
	}
	if isAdminIdentity("admin4@example.com") {
		t.Error("複数設定に含まれないメールアドレスは管理者と判定されるべきではない")
	}
}

// TestIsAdminIdentity_UsernameIgnored はユーザー名（表示名）が ADMIN_USERNAMES 環境変数に一致しても管理者昇格しないことを検証する（C-4修正の回帰テスト）
func TestIsAdminIdentity_UsernameIgnored(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "")
	t.Setenv("ADMIN_USERNAMES", "superadmin")

	// ADMIN_EMAILS に含まれないが ADMIN_USERNAMES には一致するケース
	// 修正前はこれが true を返していた（脆弱性）
	if isAdminIdentity("superadmin@attacker.com") {
		t.Error("ユーザー名による管理者昇格は廃止されているべき（C-4修正の回帰テスト）")
	}
}

// TestIsAdminIdentity_LocalPartIgnored はメールのローカルパートだけの一致で管理者昇格しないことを検証する（C-4修正の回帰テスト）
func TestIsAdminIdentity_LocalPartIgnored(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "")
	t.Setenv("ADMIN_USERNAMES", "admin")

	// 修正前は email のローカルパートが ADMIN_USERNAMES に一致すると管理者になれた
	if isAdminIdentity("admin@attacker.com") {
		t.Error("メールのローカルパートだけの一致による管理者昇格は廃止されているべき（C-4修正の回帰テスト）")
	}
}

// --- promoteAdminIfMatched テスト ---

// mockUserRepo は promoteAdminIfMatched テスト用の最小モック
type mockUserRepo struct {
	updated bool
}

func (m *mockUserRepo) CreateUser(_ *entity.User) error                                    { return nil }
func (m *mockUserRepo) GetUserByEmail(_ string) (*entity.User, error)                     { return nil, nil }
func (m *mockUserRepo) GetUserByID(_ uint) (*entity.User, error)                          { return nil, nil }
func (m *mockUserRepo) ListUsers() ([]entity.User, error)                                  { return nil, nil }
func (m *mockUserRepo) ListUsersPaged(_, _ int, _ string) ([]entity.User, int64, error)    { return nil, 0, nil }
func (m *mockUserRepo) UpdateUser(_ *entity.User) error                                    { m.updated = true; return nil }
func (m *mockUserRepo) DeleteUser(_ uint) error                                            { return nil }
func (m *mockUserRepo) GetUserByVerificationToken(_ string) (*entity.User, error)          { return nil, nil }
func (m *mockUserRepo) GetUserByPasswordResetToken(_ string) (*entity.User, error)         { return nil, nil }
func (m *mockUserRepo) GetUserByOAuth(_, _ string) (*entity.User, error)                   { return nil, errors.New("not implemented") }

// TestPromoteAdminIfMatched_EmailMatch はメール一致で管理者に昇格することを検証する
func TestPromoteAdminIfMatched_EmailMatch(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")

	user := &entity.User{ID: 1, Email: "admin@example.com", IsAdmin: false}
	repo := &mockUserRepo{}
	promoteAdminIfMatched(user, repo)

	if !user.IsAdmin {
		t.Error("ADMIN_EMAILS に含まれるメールのユーザーは管理者に昇格するべき")
	}
	if !repo.updated {
		t.Error("昇格後に UpdateUser が呼ばれるべき")
	}
}

// TestPromoteAdminIfMatched_AlreadyAdmin は既に管理者のユーザーには何もしないことを検証する
func TestPromoteAdminIfMatched_AlreadyAdmin(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")

	user := &entity.User{ID: 1, Email: "admin@example.com", IsAdmin: true}
	repo := &mockUserRepo{}
	promoteAdminIfMatched(user, repo)

	if repo.updated {
		t.Error("既に管理者のユーザーに対して UpdateUser を呼んではならない")
	}
}

// TestPromoteAdminIfMatched_NoMatch はメール不一致のユーザーは昇格しないことを検証する
func TestPromoteAdminIfMatched_NoMatch(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")

	user := &entity.User{ID: 2, Email: "user@example.com", IsAdmin: false}
	repo := &mockUserRepo{}
	promoteAdminIfMatched(user, repo)

	if user.IsAdmin {
		t.Error("ADMIN_EMAILS に含まれないメールのユーザーは管理者に昇格するべきではない")
	}
}

// TestPromoteAdminIfMatched_NameMatchDoesNotPromote はユーザー名一致だけでは昇格しないことを検証する（C-4修正の回帰テスト）
func TestPromoteAdminIfMatched_NameMatchDoesNotPromote(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "")
	t.Setenv("ADMIN_USERNAMES", "superadmin")

	// ADMIN_USERNAMES に一致する名前を持つが ADMIN_EMAILS には含まれないユーザー
	user := &entity.User{ID: 3, Email: "attacker@evil.com", Name: "superadmin", IsAdmin: false}
	repo := &mockUserRepo{}
	promoteAdminIfMatched(user, repo)

	if user.IsAdmin {
		t.Error("ユーザー名の一致だけでは管理者に昇格できてはならない（C-4修正の回帰テスト）")
	}
}
