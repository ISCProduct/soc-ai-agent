package services

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
	"os"
	"time"

	"golang.org/x/oauth2"
	"gorm.io/gorm"
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

	encKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", errors.New("google calendar not connected")
		}
		return nil, "", fmt.Errorf("failed to get token: %w", err)
	}

	encKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
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
	Summary     string              `json:"summary"`
	Description string              `json:"description,omitempty"`
	Start       calendarEventTime   `json:"start"`
	End         calendarEventTime   `json:"end"`
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

// CreateEvent はGoogleカレンダーにイベントを作成し、スケジュールのGoogleCalendarEventIDを更新する。
func (s *CalendarSyncService) CreateEvent(ctx context.Context, userID uint, ev *models.ScheduleEvent) {
	client, calID, err := s.getOAuthClient(ctx, userID)
	if err != nil {
		log.Printf("[CalendarSync] CreateEvent: user=%d %v", userID, err)
		return
	}

	body := buildEventBody(ev)
	payload, err := json.Marshal(body)
	if err != nil {
		log.Printf("[CalendarSync] CreateEvent: marshal error: %v", err)
		return
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events", calID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("[CalendarSync] CreateEvent: build request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CalendarSync] CreateEvent: http error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[CalendarSync] CreateEvent: unexpected status %d", resp.StatusCode)
		return
	}

	var result calendarEventResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.ID == "" {
		log.Printf("[CalendarSync] CreateEvent: decode response error: %v", err)
		return
	}

	ev.GoogleCalendarEventID = result.ID
	if err := s.scheduleRepo.Update(ev); err != nil {
		log.Printf("[CalendarSync] CreateEvent: failed to save event id for schedule %d: %v", ev.ID, err)
	}
}

// UpdateEvent はGoogleカレンダーのイベントを更新する。
func (s *CalendarSyncService) UpdateEvent(ctx context.Context, userID uint, ev *models.ScheduleEvent) {
	if ev.GoogleCalendarEventID == "" {
		// カレンダーイベントIDがない場合は新規作成
		s.CreateEvent(ctx, userID, ev)
		return
	}

	client, calID, err := s.getOAuthClient(ctx, userID)
	if err != nil {
		log.Printf("[CalendarSync] UpdateEvent: user=%d %v", userID, err)
		return
	}

	body := buildEventBody(ev)
	payload, err := json.Marshal(body)
	if err != nil {
		log.Printf("[CalendarSync] UpdateEvent: marshal error: %v", err)
		return
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s", calID, ev.GoogleCalendarEventID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("[CalendarSync] UpdateEvent: build request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CalendarSync] UpdateEvent: http error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[CalendarSync] UpdateEvent: unexpected status %d", resp.StatusCode)
	}
}

// DeleteEvent はGoogleカレンダーのイベントを削除する。
func (s *CalendarSyncService) DeleteEvent(ctx context.Context, userID uint, googleEventID string) {
	if googleEventID == "" {
		return
	}

	client, calID, err := s.getOAuthClient(ctx, userID)
	if err != nil {
		log.Printf("[CalendarSync] DeleteEvent: user=%d %v", userID, err)
		return
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s", calID, googleEventID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		log.Printf("[CalendarSync] DeleteEvent: build request error: %v", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CalendarSync] DeleteEvent: http error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		log.Printf("[CalendarSync] DeleteEvent: unexpected status %d", resp.StatusCode)
	}
}
