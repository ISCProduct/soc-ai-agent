package costs

import (
	"errors"
	"testing"
	"time"
)

type stubCounter struct {
	count int64
	err   error
	since time.Time
}

func (s *stubCounter) CountSince(since time.Time) (int64, error) {
	s.since = since
	return s.count, s.err
}

// 監視の失敗で面接を止めないこと。
// 集計できないことと予算超過は別で、前者で学生を弾いてはいけない。
func TestAllowStart_CountFailureDoesNotBlock(t *testing.T) {
	svc := NewInterviewBudgetService(&stubCounter{err: errors.New("db down")}, nil)
	if err := svc.AllowStart(); err != nil {
		t.Errorf("集計失敗で面接を止めた: %v", err)
	}
}

// サービス未設定でも止めないこと。
func TestAllowStart_NilSafe(t *testing.T) {
	var nilSvc *InterviewBudgetService
	if err := nilSvc.AllowStart(); err != nil {
		t.Errorf("nil で止めた: %v", err)
	}
	if err := NewInterviewBudgetService(nil, nil).AllowStart(); err != nil {
		t.Errorf("repo 未設定で止めた: %v", err)
	}
}

// 既定では止めない。実際の利用量が見えないまま遮断すると、
// 面接練習をしたい学生を理由も分からず弾いてしまう。
func TestAllowStart_DefaultIsMonitorOnly(t *testing.T) {
	t.Setenv("INTERVIEW_MONTHLY_LIMIT", "10")
	t.Setenv("INTERVIEW_BUDGET_ENFORCE", "")
	svc := NewInterviewBudgetService(&stubCounter{count: 999}, nil)
	if svc.enforce {
		t.Error("既定で enforce が有効になっている")
	}
	if err := svc.AllowStart(); err != nil {
		t.Errorf("監視のみのはずが止めた: %v", err)
	}
}

// enforce を明示したときだけ止める。
func TestAllowStart_EnforceBlocksOverLimit(t *testing.T) {
	t.Setenv("INTERVIEW_MONTHLY_LIMIT", "10")
	t.Setenv("INTERVIEW_BUDGET_ENFORCE", "true")

	tests := []struct {
		name      string
		count     int64
		wantBlock bool
	}{
		{"上限未満", 9, false},
		{"上限ちょうど", 10, true},
		{"上限超過", 11, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInterviewBudgetService(&stubCounter{count: tt.count}, nil)
			err := svc.AllowStart()
			if tt.wantBlock && !errors.Is(err, ErrInterviewBudgetExceeded) {
				t.Errorf("上限超過を通した: %v", err)
			}
			if !tt.wantBlock && err != nil {
				t.Errorf("上限内なのに止めた: %v", err)
			}
		})
	}
}

// 超過してから気づくのでは遅いので、既定は上限の8割で通知する。
func TestAllowStart_NotifiesBeforeLimit(t *testing.T) {
	t.Setenv("INTERVIEW_MONTHLY_LIMIT", "100")
	t.Setenv("INTERVIEW_ALERT_THRESHOLD", "")

	var got []InterviewBudgetStatus
	svc := NewInterviewBudgetService(&stubCounter{count: 80}, func(s InterviewBudgetStatus) {
		got = append(got, s)
	})
	if svc.alertThreshold != 80 {
		t.Errorf("通知閾値 = %d, want 80（上限の8割）", svc.alertThreshold)
	}
	if err := svc.AllowStart(); err != nil {
		t.Fatalf("通知の段階で止めた: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("通知回数 = %d, want 1", len(got))
	}
	if got[0].Count != 80 || got[0].Limit != 100 {
		t.Errorf("通知内容が不正: %+v", got[0])
	}
}

// 同じ月に何度も通知しない。毎回の面接開始で通知が飛ぶと無視される。
func TestNotify_OncePerMonth(t *testing.T) {
	t.Setenv("INTERVIEW_MONTHLY_LIMIT", "10")
	count := 0
	svc := NewInterviewBudgetService(&stubCounter{count: 100}, func(InterviewBudgetStatus) { count++ })
	for range 5 {
		_ = svc.AllowStart()
	}
	if count != 1 {
		t.Errorf("通知回数 = %d, want 1", count)
	}
}

// 当月の頭から数えること。過去分まで数えると即座に上限に達する。
func TestStatus_CountsFromMonthStart(t *testing.T) {
	stub := &stubCounter{count: 5}
	svc := NewInterviewBudgetService(stub, nil)
	status, err := svc.Status()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	now := time.Now().UTC()
	if stub.since.Day() != 1 || stub.since.Month() != now.Month() || stub.since.Year() != now.Year() {
		t.Errorf("集計開始 = %v, want 当月1日", stub.since)
	}
	if stub.since.Hour() != 0 || stub.since.Minute() != 0 {
		t.Errorf("集計開始が0時ちょうどでない: %v", stub.since)
	}
	if status.Month != now.Format("2006-01") {
		t.Errorf("Month = %q", status.Month)
	}
}

// 残枠は負にしない。画面に -3 と出ても意味が無い。
func TestStatus_RemainingNeverNegative(t *testing.T) {
	t.Setenv("INTERVIEW_MONTHLY_LIMIT", "10")
	svc := NewInterviewBudgetService(&stubCounter{count: 25}, nil)
	status, err := svc.Status()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if status.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", status.Remaining)
	}
	if !status.Exceeded {
		t.Error("Exceeded が false")
	}
}

// 不正な上限値で 0 除算や即遮断にならないこと。
func TestNewInterviewBudgetService_InvalidLimit(t *testing.T) {
	for _, v := range []string{"0", "-5", "abc"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("INTERVIEW_MONTHLY_LIMIT", v)
			svc := NewInterviewBudgetService(&stubCounter{count: 1}, nil)
			if svc.limit != defaultInterviewMonthlyLimit {
				t.Errorf("limit = %d, want %d", svc.limit, defaultInterviewMonthlyLimit)
			}
		})
	}
}
