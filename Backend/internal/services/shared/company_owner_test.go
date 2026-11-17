package shared

import (
	"errors"
	"testing"
)

type fakeAccess struct {
	admin, owns bool
	err         error
	ownsCalls   int
}

func (f *fakeAccess) IsAdmin(uint) (bool, error) { return f.admin, f.err }
func (f *fakeAccess) OwnsCompany(uint, uint) (bool, error) {
	f.ownsCalls++
	return f.owns, f.err
}

// SQL の検証は repositories.CompanyAccessRepository のテスト側にある。
// ここは「読み出し元が無いときは拒否する」(fail-closed)側を見る。

func TestUserIsAdmin_NilReaderIsFalse(t *testing.T) {
	ok, err := UserIsAdmin(nil, 1)
	if err != nil || ok {
		t.Fatalf("got ok=%v err=%v want false,nil", ok, err)
	}
}

func TestUserOwnsCompany_NilReaderOrZeroCompanyIsFalse(t *testing.T) {
	ok, err := UserOwnsCompany(nil, 1, 10)
	if err != nil || ok {
		t.Fatalf("nil reader: got ok=%v err=%v", ok, err)
	}
	f := &fakeAccess{owns: true}
	ok, err = UserOwnsCompany(f, 1, 0)
	if err != nil || ok {
		t.Fatalf("companyID=0: got ok=%v err=%v", ok, err)
	}
	if f.ownsCalls != 0 {
		t.Fatal("companyID=0 のときは問い合わせない")
	}
}

func TestUserAccess_DelegatesToReader(t *testing.T) {
	f := &fakeAccess{admin: true, owns: true}
	if ok, err := UserIsAdmin(f, 7); err != nil || !ok {
		t.Fatalf("UserIsAdmin: ok=%v err=%v", ok, err)
	}
	if ok, err := UserOwnsCompany(f, 1, 10); err != nil || !ok {
		t.Fatalf("UserOwnsCompany: ok=%v err=%v", ok, err)
	}
	boom := errors.New("db down")
	if _, err := UserOwnsCompany(&fakeAccess{err: boom}, 1, 10); !errors.Is(err, boom) {
		t.Fatalf("エラーをそのまま返すこと: %v", err)
	}
}
