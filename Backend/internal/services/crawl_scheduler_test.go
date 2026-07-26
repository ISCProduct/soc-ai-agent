package services

import (
	"Backend/internal/models"
	"testing"
	"time"
)

func TestComputeNextRun(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)

	tests := []struct {
		name   string
		now    time.Time
		source models.CrawlSource
		want   string // RFC3339 in loc
	}{
		{
			name: "daily: 当日未到来なら当日",
			now:  time.Date(2024, 1, 15, 1, 0, 0, 0, loc),
			source: models.CrawlSource{
				ScheduleType: "daily",
				ScheduleTime: "02:00",
			},
			want: "2024-01-15T02:00:00+09:00",
		},
		{
			name: "daily: 当日過ぎたら翌日",
			now:  time.Date(2024, 1, 15, 3, 0, 0, 0, loc),
			source: models.CrawlSource{
				ScheduleType: "daily",
				ScheduleTime: "02:00",
			},
			want: "2024-01-16T02:00:00+09:00",
		},
		{
			name: "weekly: 同曜日で時刻過ぎたら翌週",
			now:  time.Date(2024, 1, 15, 3, 0, 0, 0, loc), // Monday
			source: models.CrawlSource{
				ScheduleType: "weekly",
				ScheduleDay:  int(time.Monday),
				ScheduleTime: "02:00",
			},
			want: "2024-01-22T02:00:00+09:00",
		},
		{
			name: "monthly: 同月内で未到来",
			now:  time.Date(2024, 1, 10, 1, 0, 0, 0, loc),
			source: models.CrawlSource{
				ScheduleType: "monthly",
				ScheduleDay:  15,
				ScheduleTime: "02:00",
			},
			want: "2024-01-15T02:00:00+09:00",
		},
		{
			name: "monthly: 31日指定で1月末→2月末(29日・閏年)",
			now:  time.Date(2024, 1, 31, 3, 0, 0, 0, loc),
			source: models.CrawlSource{
				ScheduleType: "monthly",
				ScheduleDay:  31,
				ScheduleTime: "02:00",
			},
			want: "2024-02-29T02:00:00+09:00",
		},
		{
			name: "monthly: 31日指定で1月末→2月末(28日・平年)",
			now:  time.Date(2023, 1, 31, 3, 0, 0, 0, loc),
			source: models.CrawlSource{
				ScheduleType: "monthly",
				ScheduleDay:  31,
				ScheduleTime: "02:00",
			},
			want: "2023-02-28T02:00:00+09:00",
		},
		{
			name: "monthly: 3/31 過ぎ → 4/30（5月スキップしない）",
			now:  time.Date(2024, 3, 31, 3, 0, 0, 0, loc),
			source: models.CrawlSource{
				ScheduleType: "monthly",
				ScheduleDay:  31,
				ScheduleTime: "02:00",
			},
			want: "2024-04-30T02:00:00+09:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeNextRun(tt.now, &tt.source)
			if got == nil {
				t.Fatal("computeNextRun returned nil")
			}
			if got.Format(time.RFC3339) != tt.want {
				t.Fatalf("got %s, want %s", got.Format(time.RFC3339), tt.want)
			}
		})
	}
}

func TestComputeNextRun_InvalidTime(t *testing.T) {
	now := time.Now()
	if got := computeNextRun(now, &models.CrawlSource{ScheduleType: "daily", ScheduleTime: "bad"}); got != nil {
		t.Fatalf("expected nil for invalid time, got %v", got)
	}
	if got := computeNextRun(now, nil); got != nil {
		t.Fatalf("expected nil for nil source, got %v", got)
	}
}
