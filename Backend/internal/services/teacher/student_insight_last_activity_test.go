package teacher

import (
	"errors"
	"testing"
	"time"
)

type stubLastActivity struct {
	byUser map[uint]time.Time
	err    error
}

func (s *stubLastActivity) FindLastActivityByUsers(_ []uint) (map[uint]time.Time, error) {
	return s.byUser, s.err
}

var insightNow = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

func lastActivityService(r LastActivityReader) *StudentInsightService {
	svc := resumeService(nil, threeStudents...)
	if r != nil {
		svc.SetLastActivityReader(r)
	}
	svc.now = func() time.Time { return insightNow }
	return svc
}

// 未活動の絞り込み（#1573）。1: 3日前に活動 / 2: 14日ちょうど前 / 3: 記録なし。
func TestListTendencies_InactiveDays(t *testing.T) {
	reader := &stubLastActivity{byUser: map[uint]time.Time{
		1: insightNow.Add(-3 * 24 * time.Hour),
		2: insightNow.Add(-14 * 24 * time.Hour),
	}}

	tests := []struct {
		name   string
		reader LastActivityReader
		days   int
		want   []uint
	}{
		{"絞り込みなしは全員", reader, 0, []uint{1, 2, 3}},
		{"14日: 境界ちょうどと記録なしを含む", reader, 14, []uint{2, 3}},
		{"30日: 記録なしだけ", reader, 30, []uint{3}},
		{"1日: 全員が未活動", reader, 1, []uint{1, 2, 3}},
		// 判定材料が無いのに生徒を隠すと、教員から見て「該当なし」と誤読される
		{"読み出し未注入なら絞らない", nil, 14, []uint{1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := lastActivityService(tt.reader).ListTendenciesFiltered(25, 0, "", nil, TendencyFilters{InactiveDays: tt.days})
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			got := make([]uint, 0, len(res.Students))
			for _, s := range res.Students {
				got = append(got, s.UserID)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("students = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("students = %v, want %v", got, tt.want)
				}
			}
			if res.Total != int64(len(tt.want)) {
				t.Fatalf("total = %d, want %d（絞り込み後の実数でページングする）", res.Total, len(tt.want))
			}
		})
	}
}

// 一覧に最終活動日時が載り、記録なしは nil のまま。
func TestListTendencies_LastActivityAtField(t *testing.T) {
	at := insightNow.Add(-48 * time.Hour)
	res, err := lastActivityService(&stubLastActivity{byUser: map[uint]time.Time{1: at}}).ListTendencies(25, 0, "", nil)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if res.Students[0].LastActivityAt == nil || !res.Students[0].LastActivityAt.Equal(at) {
		t.Fatalf("user1 LastActivityAt = %v, want %v", res.Students[0].LastActivityAt, at)
	}
	if res.Students[2].LastActivityAt != nil {
		t.Fatalf("記録なしの生徒に日時が付いた: %v", res.Students[2].LastActivityAt)
	}
}

// 読み出しの失敗を「全員未活動」として返さない。
func TestListTendencies_LastActivityErrorPropagates(t *testing.T) {
	_, err := lastActivityService(&stubLastActivity{err: errors.New("db down")}).ListTendenciesFiltered(25, 0, "", nil, TendencyFilters{InactiveDays: 14})
	if err == nil {
		t.Fatal("エラーが握りつぶされた")
	}
}
