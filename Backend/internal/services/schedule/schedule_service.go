package schedule

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"Backend/internal/safego"
)

// CalendarSyncer は予定の作成・更新・削除を Google カレンダーへ伝える。
// 失敗は error で返す。呼び出し側がログと再試行を持つ。
type CalendarSyncer interface {
	CreateEvent(ctx context.Context, userID uint, ev *models.ScheduleEvent) error
	UpdateEvent(ctx context.Context, userID uint, ev *models.ScheduleEvent) error
	DeleteEvent(ctx context.Context, userID uint, googleEventID string) error
}

type ScheduleService struct {
	repo         repository.ScheduleRepository
	calendarSync CalendarSyncer
}

func NewScheduleService(repo repository.ScheduleRepository) *ScheduleService {
	return &ScheduleService{repo: repo}
}

// SetCalendarSyncService はGoogleカレンダー同期サービスを注入する（オプション）。
func (s *ScheduleService) SetCalendarSyncService(svc CalendarSyncer) {
	s.calendarSync = svc
}

func (s *ScheduleService) Create(userID uint, companyName, stage, title string, scheduledAt time.Time, notes string) (*models.ScheduleEvent, error) {
	if strings.TrimSpace(companyName) == "" {
		return nil, errors.New("company_name is required")
	}
	if scheduledAt.IsZero() {
		return nil, errors.New("scheduled_at is required")
	}
	event := &models.ScheduleEvent{
		UserID:      userID,
		CompanyName: companyName,
		Stage:       models.ScheduleStage(stage),
		Title:       title,
		ScheduledAt: scheduledAt,
		Notes:       notes,
	}
	if err := s.repo.Create(event); err != nil {
		return nil, err
	}
	// Googleカレンダーへの同期は非同期で実行（スケジュール登録の成否には影響しない）
	if s.calendarSync != nil {
		// カレンダー同期は投げっぱなし。panic でプロセスを落とさない(#1446)。
		// 一時的な失敗は数回やり直す。黙って捨てると DB とカレンダーがずれたままになる。
		ev := event
		safego.Go(func() {
			syncCalendar("CreateEvent", userID, func() error {
				return s.calendarSync.CreateEvent(context.Background(), userID, ev)
			})
		})
	}
	return event, nil
}

func (s *ScheduleService) Get(userID, eventID uint) (*models.ScheduleEvent, error) {
	event, err := s.repo.FindByID(eventID)
	if err != nil {
		return nil, err
	}
	if event.UserID != userID {
		return nil, errors.New("forbidden")
	}
	return event, nil
}

func (s *ScheduleService) Update(userID, eventID uint, companyName, stage, title string, scheduledAt time.Time, notes string) (*models.ScheduleEvent, error) {
	event, err := s.repo.FindByID(eventID)
	if err != nil {
		return nil, err
	}
	if event.UserID != userID {
		return nil, errors.New("forbidden")
	}
	if strings.TrimSpace(companyName) != "" {
		event.CompanyName = companyName
	}
	if stage != "" {
		event.Stage = models.ScheduleStage(stage)
	}
	event.Title = title
	if !scheduledAt.IsZero() {
		event.ScheduledAt = scheduledAt
	}
	event.Notes = notes
	if err := s.repo.Update(event); err != nil {
		return nil, err
	}
	if s.calendarSync != nil {
		ev := event
		safego.Go(func() {
			syncCalendar("UpdateEvent", userID, func() error {
				return s.calendarSync.UpdateEvent(context.Background(), userID, ev)
			})
		})
	}
	return event, nil
}

func (s *ScheduleService) Delete(userID, eventID uint) error {
	event, err := s.repo.FindByID(eventID)
	if err != nil {
		return err
	}
	if event.UserID != userID {
		return errors.New("forbidden")
	}
	googleEventID := event.GoogleCalendarEventID
	if err := s.repo.Delete(eventID); err != nil {
		return err
	}
	if s.calendarSync != nil && googleEventID != "" {
		safego.Go(func() {
			syncCalendar("DeleteEvent", userID, func() error {
				return s.calendarSync.DeleteEvent(context.Background(), userID, googleEventID)
			})
		})
	}
	return nil
}

func (s *ScheduleService) List(userID uint) ([]models.ScheduleEvent, error) {
	return s.repo.ListByUser(userID)
}

func (s *ScheduleService) ListByRange(userID uint, from, to time.Time) ([]models.ScheduleEvent, error) {
	return s.repo.ListByUserAndRange(userID, from, to)
}

// ExportICS は指定ユーザーの全スケジュールを iCalendar 形式で返す。
func (s *ScheduleService) ExportICS(userID uint) (string, error) {
	events, err := s.repo.ListByUser(userID)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//soc-ai-agent//Schedule//JA\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")

	for _, ev := range events {
		title := ev.Title
		if title == "" {
			title = fmt.Sprintf("%s - %s", ev.CompanyName, ev.Stage)
		}
		dtStart := ev.ScheduledAt.UTC().Format("20060102T150405Z")
		dtEnd := ev.ScheduledAt.UTC().Add(time.Hour).Format("20060102T150405Z")
		uid := fmt.Sprintf("schedule-%d@soc-ai-agent", ev.ID)

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(foldICSLine("UID:" + uid))
		b.WriteString(foldICSLine("DTSTART:" + dtStart))
		b.WriteString(foldICSLine("DTEND:" + dtEnd))
		b.WriteString(foldICSLine("SUMMARY:" + escapeICS(title)))
		if ev.Notes != "" {
			b.WriteString(foldICSLine("DESCRIPTION:" + escapeICS(ev.Notes)))
		}
		b.WriteString(foldICSLine("CATEGORIES:" + escapeICS(string(ev.Stage))))
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String(), nil
}

// foldICSLine は RFC 5545 に従い、75オクテット超の行を CRLF + スペースで折り返す
func foldICSLine(line string) string {
	const maxOctets = 75
	var b strings.Builder
	octets := 0
	for _, r := range line {
		encoded := []byte(string(r))
		if octets+len(encoded) > maxOctets {
			b.WriteString("\r\n ")
			octets = 1 // 折り返しの先頭スペース分
		}
		b.WriteRune(r)
		octets += len(encoded)
	}
	b.WriteString("\r\n")
	return b.String()
}

const calendarSyncAttempts = 3

// calendarSyncPause は再試行の待ち。テストはここを差し替える。
var calendarSyncPause = func(attempt int) {
	time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
}

// syncCalendar は一時的な失敗だけやり直す。連携していない等の恒久的な失敗は1回で止める。
func syncCalendar(op string, userID uint, fn func() error) {
	var err error
	for attempt := 1; attempt <= calendarSyncAttempts; attempt++ {
		err = fn()
		if err == nil {
			return
		}
		if !isRetryableCalendar(err) || attempt == calendarSyncAttempts {
			log.Printf("[CalendarSync] %s: user=%d failed (attempt %d/%d): %v", op, userID, attempt, calendarSyncAttempts, err)
			return
		}
		log.Printf("[CalendarSync] %s: user=%d will retry (attempt %d/%d): %v", op, userID, attempt, calendarSyncAttempts, err)
		calendarSyncPause(attempt)
	}
}

func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// FoldICSLine is an exported wrapper for foldICSLine for use in external tests.
func FoldICSLine(line string) string { return foldICSLine(line) }
