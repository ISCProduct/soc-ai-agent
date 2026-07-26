package services

import (
	"Backend/internal/models"
	"time"
)

func (s *CrawlService) StartScheduler() {
	s.EnsureL1WarmCrawlSource()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.runDueSources()
	}
}

func (s *CrawlService) RunDueSources() {
	s.runDueSources()
}

func (s *CrawlService) runDueSources() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	sources, err := s.repo.ListDueSources(now)
	if err != nil {
		return
	}
	for i := range sources {
		_, _ = s.runSource(&sources[i])
	}
}

func isValidTime(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}

func computeNextRun(now time.Time, source *models.CrawlSource) *time.Time {
	if source == nil {
		return nil
	}
	hourMin, err := time.Parse("15:04", source.ScheduleTime)
	if err != nil {
		return nil
	}
	hour := hourMin.Hour()
	min := hourMin.Minute()
	loc := now.Location()
	var next time.Time
	switch source.ScheduleType {
	case "daily":
		next = time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
	case "weekly":
		target := time.Weekday(source.ScheduleDay)
		days := (int(target) - int(now.Weekday()) + 7) % 7
		next = time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc).AddDate(0, 0, days)
		if !next.After(now) {
			next = next.AddDate(0, 0, 7)
		}
	default: // monthly
		day := source.ScheduleDay
		year, month := now.Year(), now.Month()
		lastDay := lastDayOfMonth(year, month, loc)
		if day > lastDay {
			day = lastDay
		}
		next = time.Date(year, month, day, hour, min, 0, 0, loc)
		if !next.After(now) {
			nextMonth := next.AddDate(0, 1, 0)
			year, month = nextMonth.Year(), nextMonth.Month()
			day = source.ScheduleDay
			lastDay = lastDayOfMonth(year, month, loc)
			if day > lastDay {
				day = lastDay
			}
			next = time.Date(year, month, day, hour, min, 0, 0, loc)
		}
	}
	return &next
}

func lastDayOfMonth(year int, month time.Month, loc *time.Location) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
}
