package services

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ScraperSessionService struct {
	repo *repositories.ScraperSessionRepository
}

func NewScraperSessionService(repo *repositories.ScraperSessionRepository) *ScraperSessionService {
	return &ScraperSessionService{repo: repo}
}

type ScraperSessionPayload struct {
	SiteKey   string     `json:"site_key"`
	Cookies   string     `json:"cookies"`    // "key=value; key2=value2" または JSON 配列
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (s *ScraperSessionService) List() ([]models.ScraperSession, error) {
	return s.repo.List()
}

func (s *ScraperSessionService) Upsert(payload ScraperSessionPayload) (*models.ScraperSession, error) {
	if strings.TrimSpace(payload.SiteKey) == "" {
		return nil, errors.New("site_key is required")
	}
	if strings.TrimSpace(payload.Cookies) == "" {
		return nil, errors.New("cookies is required")
	}

	// Cookie文字列をJSON配列に正規化
	normalized, err := normalizeCookies(payload.Cookies)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize cookies: %w", err)
	}

	session := &models.ScraperSession{
		SiteKey:   strings.TrimSpace(payload.SiteKey),
		Cookies:   normalized,
		ExpiresAt: payload.ExpiresAt,
	}
	if err := s.repo.Upsert(session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ScraperSessionService) Delete(siteKey string) error {
	if strings.TrimSpace(siteKey) == "" {
		return errors.New("site_key is required")
	}
	return s.repo.DeleteBySiteKey(siteKey)
}

// GetCookiesForSite 指定サイトのCookieスライスを返す（未登録・期限切れの場合はnil）
func (s *ScraperSessionService) GetCookiesForSite(siteKey string) []models.ScraperCookie {
	session, err := s.repo.GetBySiteKey(siteKey)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[ScraperSession] failed to get session for %s: %v", siteKey, err)
		}
		return nil
	}
	if session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now()) {
		log.Printf("[ScraperSession] session for %s is expired", siteKey)
		return nil
	}
	var cookies []models.ScraperCookie
	if err := json.Unmarshal([]byte(session.Cookies), &cookies); err != nil {
		log.Printf("[ScraperSession] failed to unmarshal cookies for %s: %v", siteKey, err)
		return nil
	}
	return cookies
}

// normalizeCookies Cookie文字列（"key=value; key2=value2"またはJSON配列）をJSON配列文字列に正規化する
func normalizeCookies(raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	// すでにJSON配列形式かチェック
	if strings.HasPrefix(raw, "[") {
		var cookies []models.ScraperCookie
		if err := json.Unmarshal([]byte(raw), &cookies); err == nil {
			b, err := json.Marshal(cookies)
			return string(b), err
		}
	}

	// "key=value; key2=value2" 形式をパース
	var cookies []models.ScraperCookie
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		cookies = append(cookies, models.ScraperCookie{
			Name:  strings.TrimSpace(part[:idx]),
			Value: strings.TrimSpace(part[idx+1:]),
		})
	}
	if len(cookies) == 0 {
		return "", errors.New("no valid cookies found in input")
	}
	b, err := json.Marshal(cookies)
	return string(b), err
}

// fetchTextWithCookies Cookieを付与してURLのテキストを取得する
func fetchTextWithCookies(url string, cookies []models.ScraperCookie) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")

	for _, c := range cookies {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
