package teacher

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/repositories"
)

type stubLowMatch struct {
	byUser       map[uint][]repositories.LowMatchApplication
	err          error
	calls        int
	gotIDs       []uint
	gotThreshold float64
}

func (s *stubLowMatch) FindLowMatchApplicationsByUsers(userIDs []uint, threshold float64) (map[uint][]repositories.LowMatchApplication, error) {
	s.calls++
	s.gotIDs = userIDs
	s.gotThreshold = threshold
	return s.byUser, s.err
}

func lowMatchService(low *stubLowMatch, students ...entity.User) *StudentInsightService {
	svc := NewStudentInsightService(
		&stubLister{students: students, total: int64(len(students))},
		&stubScores{scores: map[uint]map[string]float64{}},
		&stubIndustries{},
		&stubProfiles{},
	)
	if low != nil {
		svc.SetLowMatchReader(low)
	}
	return svc
}

var threeStudents = []entity.User{
	{ID: 1, Name: "山田", Email: "y@example.com"},
	{ID: 2, Name: "鈴木", Email: "s@example.com"},
	{ID: 3, Name: "田中", Email: "t@example.com"},
}

// 低マッチ応募がある生徒だけに絞れること（#1028）。
func TestListTendenciesLowMatchOnly_FiltersStudents(t *testing.T) {
	low := &stubLowMatch{byUser: map[uint][]repositories.LowMatchApplication{
		2: {{UserID: 2, CompanyName: "サンプル商事", MatchScore: 21.5, Status: "applied"}},
	}}
	res, err := lowMatchService(low, threeStudents...).ListTendenciesLowMatchOnly(25, 0, "", nil)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Students) != 1 || res.Students[0].UserID != 2 {
		t.Fatalf("該当生徒のみ返すこと: %+v", res.Students)
	}
	if len(res.Students[0].LowMatchApplications) != 1 {
		t.Errorf("応募内容が付いていない: %+v", res.Students[0])
	}
	// 総数を絞り込み後の実数にしないと「0件なのに総数3」と表示される
	if res.Total != 1 {
		t.Errorf("Total = %d, want 1", res.Total)
	}
}

// 絞り込みなしでも低マッチ応募は付く。
// 一覧上で気づける方が、フィルタを掛け直すより有用。
func TestListTendencies_AttachesLowMatchWithoutFilter(t *testing.T) {
	low := &stubLowMatch{byUser: map[uint][]repositories.LowMatchApplication{
		2: {{UserID: 2, CompanyName: "サンプル商事", MatchScore: 21.5, Status: "applied"}},
	}}
	res, err := lowMatchService(low, threeStudents...).ListTendencies(25, 0, "", nil)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Students) != 3 {
		t.Fatalf("絞り込みなしでは全員返すこと: %d", len(res.Students))
	}
	for _, s := range res.Students {
		if s.UserID == 2 && len(s.LowMatchApplications) != 1 {
			t.Errorf("該当生徒に応募内容が付いていない: %+v", s)
		}
		if s.UserID != 2 && len(s.LowMatchApplications) != 0 {
			t.Errorf("該当しない生徒に応募が付いている: %+v", s)
		}
	}
}

// 生徒数によらず1回しか呼ばない（N+1にしない）。
func TestListTendencies_LowMatchIsBatched(t *testing.T) {
	low := &stubLowMatch{}
	if _, err := lowMatchService(low, threeStudents...).ListTendencies(25, 0, "", nil); err != nil {
		t.Fatalf("error = %v", err)
	}
	if low.calls != 1 {
		t.Errorf("呼び出し回数 = %d, want 1 (生徒ごとに引くと N+1)", low.calls)
	}
	if len(low.gotIDs) != 3 {
		t.Errorf("全生徒分をまとめて渡すこと: %v", low.gotIDs)
	}
}

// 閾値がサービス定数として渡ること。
func TestListTendencies_PassesThreshold(t *testing.T) {
	low := &stubLowMatch{}
	if _, err := lowMatchService(low, threeStudents...).ListTendencies(25, 0, "", nil); err != nil {
		t.Fatalf("error = %v", err)
	}
	if low.gotThreshold != LowMatchThreshold {
		t.Errorf("threshold = %v, want %v", low.gotThreshold, LowMatchThreshold)
	}
}

// 未注入でも落ちない（既存構成との互換）。
func TestListTendencies_NoLowMatchReader(t *testing.T) {
	res, err := lowMatchService(nil, threeStudents...).ListTendencies(25, 0, "", nil)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Students) != 3 {
		t.Errorf("生徒数 = %d, want 3", len(res.Students))
	}
}

// 読み出し失敗を握り潰さない。握り潰すと「低マッチ応募なし」と区別できない。
func TestListTendencies_LowMatchError(t *testing.T) {
	low := &stubLowMatch{err: errors.New("db down")}
	if _, err := lowMatchService(low, threeStudents...).ListTendencies(25, 0, "", nil); err == nil {
		t.Error("error = nil, want error")
	}
}

// 学生側(frontend/lib/low-match.ts)と閾値が食い違うと、
// 「学生には確認が出ないのに教員一覧には出る」ことになる。
//
// 学生側は PR #1230 で入る。両方が揃った develop では突き合わせが走り、
// 片方だけのブランチではスキップする。
func TestLowMatchThreshold_MatchesFrontend(t *testing.T) {
	// go test の作業ディレクトリはパッケージのディレクトリ
	const frontendPath = "../../../../frontend/lib/low-match.ts"
	src, err := os.ReadFile(frontendPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("学生側(#1230)がまだ入っていないためスキップ")
		}
		t.Fatalf("フロント側の閾値定義を読めない: %v", err)
	}
	m := regexp.MustCompile(`LOW_MATCH_THRESHOLD = (\d+(?:\.\d+)?)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("フロント側の LOW_MATCH_THRESHOLD を見つけられない")
	}
	want, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		t.Fatalf("パースできない: %v", err)
	}
	if LowMatchThreshold != want {
		t.Errorf("閾値が食い違っている: backend=%v frontend=%v", LowMatchThreshold, want)
	}
}
