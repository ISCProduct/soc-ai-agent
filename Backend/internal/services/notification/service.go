// Package notification は個人宛のアプリ内通知（#2033）を扱う。
//
// 締切・選考結果・スカウト・先生の案内を1か所に集め、ヘッダのベルと未読バッジで見落としを防ぐ。
// 通知の作成は本来の処理（応募の更新・スカウトの送信など）の付け足しなので、失敗しても
// 本来の処理は止めず、ログだけ残す（Notify はエラーを返さない）。
package notification

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"Backend/internal/models"
)

// ErrNotFound は本人の通知が見つからない（他人の通知を含む）ことを表す。
var ErrNotFound = errors.New("notification not found")

// Notice は作成する通知の内容。
type Notice struct {
	UserID uint
	Kind   string
	Title  string
	Body   string
	Link   string
	// DedupeKey は同じイベントの二重通知を防ぐキー。空なら重複を見ない。
	DedupeKey string
}

// Notifier は通知を作る側（応募・スカウト・案内）が使う。
type Notifier interface {
	Notify(n Notice)
}

// Store は通知の永続化（repositories.NotificationRepository が満たす）。
type Store interface {
	Create(n *models.Notification) error
	ListForUser(userID uint, limit int, beforeID uint) ([]models.Notification, error)
	UnreadCount(userID uint) (int64, error)
	MarkRead(userID, id uint, at time.Time) (int64, error)
	MarkAllRead(userID uint, at time.Time) error
	ScheduleEventsBetween(from, to time.Time) ([]models.ScheduleEvent, error)
}

// Service はアプリ内通知のサービス。
type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

const (
	maxTitleRunes = 200
	maxBodyRunes  = 1000
	maxLinkRunes  = 500
	// MaxListLimit は一覧で一度に返す最大件数。
	MaxListLimit = 50
)

func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// Notify は通知を1件作る。宛先・種類・見出しが無ければ作らない。失敗してもログだけ残す。
func (s *Service) Notify(n Notice) {
	if s == nil || s.store == nil || n.UserID == 0 || n.Kind == "" || strings.TrimSpace(n.Title) == "" {
		return
	}
	row := &models.Notification{
		UserID: n.UserID,
		Kind:   n.Kind,
		Title:  truncateRunes(n.Title, maxTitleRunes),
		Body:   truncateRunes(n.Body, maxBodyRunes),
		Link:   truncateRunes(n.Link, maxLinkRunes),
	}
	if key := strings.TrimSpace(n.DedupeKey); key != "" {
		row.DedupeKey = &key
	}
	if err := s.store.Create(row); err != nil {
		log.Printf("[Notification] 作成に失敗 user_id=%d kind=%s: %v", n.UserID, n.Kind, err)
	}
}

// List は本人の通知を新しい順に返す。
func (s *Service) List(userID uint, limit int, beforeID uint) ([]models.Notification, error) {
	if limit <= 0 || limit > MaxListLimit {
		limit = MaxListLimit
	}
	return s.store.ListForUser(userID, limit, beforeID)
}

// UnreadCount は本人の未読の件数を返す。
func (s *Service) UnreadCount(userID uint) (int64, error) {
	return s.store.UnreadCount(userID)
}

// MarkRead は本人の通知1件を既読にする。他人の通知や存在しない通知は ErrNotFound（既読済みも同じ扱いにしない）。
func (s *Service) MarkRead(userID, id uint) error {
	n, err := s.store.MarkRead(userID, id, s.now())
	if err != nil {
		return err
	}
	if n == 0 {
		// 既読済みの本人の通知も0件になる。冪等にするため、本人の通知かどうかだけを見直す
		rows, err := s.store.ListForUser(userID, 1, id+1)
		if err != nil {
			return err
		}
		if len(rows) == 0 || rows[0].ID != id {
			return ErrNotFound
		}
	}
	return nil
}

// MarkAllRead は本人の未読をすべて既読にする。
func (s *Service) MarkAllRead(userID uint) error {
	return s.store.MarkAllRead(userID, s.now())
}

// NotifyUpcomingSchedules は、これから24時間以内に予定（締切・面接など）がある学生へ前日の通知を出す。
// 定期処理から何度呼んでも、予定ごと（日時を変えたら別）に1件だけになる。
func (s *Service) NotifyUpcomingSchedules() {
	now := s.now()
	events, err := s.store.ScheduleEventsBetween(now, now.Add(24*time.Hour))
	if err != nil {
		log.Printf("[Notification] 予定の取得に失敗: %v", err)
		return
	}
	for _, ev := range events {
		title := strings.TrimSpace(ev.Title)
		if title == "" {
			title = "予定"
		}
		s.Notify(Notice{
			UserID:    ev.UserID,
			Kind:      models.NotificationKindDeadline,
			Title:     fmt.Sprintf("明日までの予定: %s", title),
			Body:      fmt.Sprintf("%s に予定があります。準備を確認しましょう。", ev.ScheduledAt.In(jst).Format("1月2日 15:04")),
			Link:      "/schedule",
			DedupeKey: fmt.Sprintf("schedule:%d:%d", ev.ID, ev.ScheduledAt.Unix()),
		})
	}
}

var jst = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("JST", 9*60*60)
}()

// applicationStatusLabels は学生に知らせる選考ステータスと、その見出し。
// 学生が自分で変えたとき（応募・辞退・承諾）は通知しない（呼び出し側で学校・企業の更新だけに絞る）。
var applicationStatusLabels = map[string]string{
	"document_screening":    "書類選考が始まりました",
	"document_passed":       "書類選考を通過しました",
	"interview_scheduled":   "面接の予定が入りました",
	"interview_in_progress": "面接の選考が進んでいます",
	"offered":               "内定の連絡がありました",
	"rejected":              "選考結果のお知らせ",
}

// ApplicationStatusNotice は学校・企業が選考ステータスを変えたときの通知を返す。知らせない変更なら ok=false。
func ApplicationStatusNotice(userID, applicationID uint, status, companyName string) (Notice, bool) {
	label, ok := applicationStatusLabels[status]
	if !ok || userID == 0 {
		return Notice{}, false
	}
	body := "応募の状況を確認しましょう。"
	if companyName != "" {
		body = companyName + " の選考状況が更新されました。応募の状況を確認しましょう。"
	}
	return Notice{
		UserID:    userID,
		Kind:      models.NotificationKindApplication,
		Title:     label,
		Body:      body,
		Link:      "/applications",
		DedupeKey: fmt.Sprintf("application:%d:%s", applicationID, status),
	}, true
}
