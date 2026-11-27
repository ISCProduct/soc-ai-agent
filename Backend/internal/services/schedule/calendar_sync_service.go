package schedule

import (
	"Backend/domain/repository"
	"Backend/internal/config"
	"Backend/internal/crypto"
	"Backend/internal/models"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// CalendarSyncService はGoogleカレンダーとの同期を管理するサービス。
type CalendarSyncService struct {
	tokenRepo    repository.UserGoogleTokenRepository
	scheduleRepo repository.ScheduleRepository
	oauthConfig  *oauth2.Config
}

func NewCalendarSyncService(tokenRepo repository.UserGoogleTokenRepository, scheduleRepo repository.ScheduleRepository, oauthCfg *config.OAuthConfig) *CalendarSyncService {
	return &CalendarSyncService{
		tokenRepo:    tokenRepo,
		scheduleRepo: scheduleRepo,
		oauthConfig:  oauthCfg.GoogleCalendar,
	}
}

// GetAuthURL はカレンダー連携用OAuth認証URLを返す。
func (s *CalendarSyncService) GetAuthURL(state string) string {
	return s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeAndSave はコードをトークンに交換して暗号化して保存する。
func (s *CalendarSyncService) ExchangeAndSave(ctx context.Context, userID uint, code string) error {
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	encKey := config.TokenEncryptionKey()
	if encKey == "" {
		encKey = "default-key-change-in-production"
	}

	encAccess, err := crypto.EncryptToken(token.AccessToken, encKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	encRefresh, err := crypto.EncryptToken(token.RefreshToken, encKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	expiry := token.Expiry
	if expiry.IsZero() {
		expiry = time.Now().Add(time.Hour)
	}

	rec := &models.UserGoogleToken{
		UserID:                userID,
		EncryptedAccessToken:  encAccess,
		EncryptedRefreshToken: encRefresh,
		Expiry:                expiry,
		CalendarID:            "primary",
	}
	return s.tokenRepo.Upsert(rec)
}

// IsConnected はユーザーがGoogleカレンダーを連携済みかどうかを返す。
func (s *CalendarSyncService) IsConnected(userID uint) bool {
	_, err := s.tokenRepo.FindByUserID(userID)
	return err == nil
}

// Disconnect は保存されているGoogleトークンを削除する。
func (s *CalendarSyncService) Disconnect(userID uint) error {
	return s.tokenRepo.DeleteByUserID(userID)
}

// getOAuthClient は保存済みトークンからOAuth HTTPクライアントを取得する（有効期限切れは自動更新）。
func (s *CalendarSyncService) getOAuthClient(ctx context.Context, userID uint) (*http.Client, string, error) {
	rec, err := s.tokenRepo.FindByUserID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", errors.New("google calendar not connected")
		}
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	encKey := config.TokenEncryptionKey()
	if encKey == "" {
		encKey = "default-key-change-in-production"
	}

	accessToken, err := crypto.DecryptToken(rec.EncryptedAccessToken, encKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt access token: %w", err)
	}
	refreshToken, err := crypto.DecryptToken(rec.EncryptedRefreshToken, encKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       rec.Expiry,
	}

	tokenSource := s.oauthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err == nil && newToken.AccessToken != accessToken {
		// アクセストークンが更新された場合は保存し直す
		newEnc, encErr := crypto.EncryptToken(newToken.AccessToken, encKey)
		if encErr == nil {
			rec.EncryptedAccessToken = newEnc
			rec.Expiry = newToken.Expiry
			if uErr := s.tokenRepo.Upsert(rec); uErr != nil {
				log.Printf("[CalendarSync] failed to update refreshed token for user %d: %v", userID, uErr)
			}
		}
	}

	return oauth2.NewClient(ctx, tokenSource), rec.CalendarID, nil
}

type calendarEventBody struct {
	Summary     string            `json:"summary"`
	Description string            `json:"description,omitempty"`
	Start       calendarEventTime `json:"start"`
	End         calendarEventTime `json:"end"`
}

type calendarEventTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type calendarEventResponse struct {
	ID string `json:"id"`
}

func buildEventBody(ev *models.ScheduleEvent) calendarEventBody {
	title := ev.Title
	if title == "" {
		title = fmt.Sprintf("%s - %s", ev.CompanyName, ev.Stage)
	}
	start := ev.ScheduledAt.Format(time.RFC3339)
	end := ev.ScheduledAt.Add(time.Hour).Format(time.RFC3339)
	return calendarEventBody{
		Summary:     title,
		Description: ev.Notes,
		Start:       calendarEventTime{DateTime: start, TimeZone: "Asia/Tokyo"},
		End:         calendarEventTime{DateTime: end, TimeZone: "Asia/Tokyo"},
	}
}

// retryableCalendarError は通信断や 429/5xx のように、同じ操作をやり直せば通る失敗。
type retryableCalendarError struct{ msg string }

func (e *retryableCalendarError) Error() string { return e.msg }

func isRetryableCalendar(err error) bool {
	var retryable *retryableCalendarError
	return errors.As(err, &retryable)
}

func calendarStatusError(status int) error {
	msg := fmt.Sprintf("unexpected status %d", status)
	if status == http.StatusTooManyRequests || status >= 500 {
		return &retryableCalendarError{msg: msg}
	}
	return errors.New(msg)
}

// CreateEvent はGoogleカレンダーにイベントを作成し、スケジュールのGoogleCalendarEventIDを更新する。
func (s *CalendarSyncService) CreateEvent(ctx context.Context, userID uint, ev *models.ScheduleEvent) error {
	client, calID, err := s.getOAuthClient(ctx, userID)
	if err != nil {
		return fmt.Errorf("user=%d %w", userID, err)
	}

	body := buildEventBody(ev)
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events", calID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return &retryableCalendarError{msg: fmt.Sprintf("http error: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return calendarStatusError(resp.StatusCode)
	}

	var result calendarEventResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.ID == "" {
		return fmt.Errorf("decode response error: %w", err)
	}

	ev.GoogleCalendarEventID = result.ID
	if err := s.scheduleRepo.Update(ev); err != nil {
		// 作成自体は成功している。やり直すとカレンダー側が二重になるので、再試行しない。
		return fmt.Errorf("failed to save event id for schedule %d: %w", ev.ID, err)
	}
	return nil
}

// UpdateEvent はGoogleカレンダーのイベントを更新する。
func (s *CalendarSyncService) UpdateEvent(ctx context.Context, userID uint, ev *models.ScheduleEvent) error {
	if ev.GoogleCalendarEventID == "" {
		// カレンダーイベントIDがない場合は新規作成
		return s.CreateEvent(ctx, userID, ev)
	}

	client, calID, err := s.getOAuthClient(ctx, userID)
	if err != nil {
		return fmt.Errorf("user=%d %w", userID, err)
	}

	body := buildEventBody(ev)
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s", calID, ev.GoogleCalendarEventID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return &retryableCalendarError{msg: fmt.Sprintf("http error: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return calendarStatusError(resp.StatusCode)
	}
	return nil
}

// DeleteEvent はGoogleカレンダーのイベントを削除する。
func (s *CalendarSyncService) DeleteEvent(ctx context.Context, userID uint, googleEventID string) error {
	if googleEventID == "" {
		return nil
	}

	client, calID, err := s.getOAuthClient(ctx, userID)
	if err != nil {
		return fmt.Errorf("user=%d %w", userID, err)
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s", calID, googleEventID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build request error: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &retryableCalendarError{msg: fmt.Sprintf("http error: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return calendarStatusError(resp.StatusCode)
	}
	return nil
}
