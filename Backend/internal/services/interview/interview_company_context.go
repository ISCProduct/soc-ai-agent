package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/company"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

const companyReadingCacheTTL = 24 * time.Hour
const companyReadingCacheMaxEntries = 1024

type companyReadingCacheEntry struct {
	value     string
	expiresAt time.Time
}

type companyReadingCache struct {
	mu      sync.Mutex
	entries map[string]companyReadingCacheEntry
}

func (c *companyReadingCache) load(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if !time.Now().Before(entry.expiresAt) {
		delete(c.entries, key)
		return "", false
	}
	return entry.value, true
}

// store は読みを記憶する。value が空でも記憶する（ネガティブキャッシュ）。
//
// lookupCompanyReading は「確実に知っている場合のみ答える」プロンプトの
// 最大15秒のLLM呼び出し。読みを返せない企業（マイナー企業・手入力の表記ゆれ）で
// 空を捨てると、フロントも company_reading="" を送り続けるため毎ターン同じ
// 呼び出しが走り、毎回課金される。STT の裏に隠れるのは STT が遅いときだけで、
// 速ければそのままターンのレイテンシに乗る。
//
// DB に読みが入れば resolveCompanyReading が先に返すので、空を覚えても取りこぼさない。
// TTL が切れれば引き直すため、あとから読みが付いた企業も拾える。
func (c *companyReadingCache) store(key, value string) {
	if key == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for cachedKey, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, cachedKey)
		}
	}
	if c.entries == nil {
		c.entries = make(map[string]companyReadingCacheEntry)
	}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= companyReadingCacheMaxEntries {
		var oldestKey string
		var oldestExpiry time.Time
		for cachedKey, entry := range c.entries {
			if oldestKey == "" || entry.expiresAt.Before(oldestExpiry) {
				oldestKey, oldestExpiry = cachedKey, entry.expiresAt
			}
		}
		delete(c.entries, oldestKey)
	}
	c.entries[key] = companyReadingCacheEntry{
		value:     strings.TrimSpace(value),
		expiresAt: now.Add(companyReadingCacheTTL),
	}
}

// resolveCompanyInfo は共有企業情報を優先し、無ければクライアント文面を使う。
// Search/LLM 調査はしない。companyType（general/sier）ではゲートしない。
func (s *InterviewService) resolveCompanyInfo(companyID uint, companyName, clientInfo string) string {
	return s.resolveCompanyInfoForPosition(companyID, companyName, "", clientInfo)
}

// resolveCompanyInfoForPosition は resolveCompanyInfo に、応募職種の求人要件を足す（#2274）。
func (s *InterviewService) resolveCompanyInfoForPosition(companyID uint, companyName, position, clientInfo string) string {
	brief := ""
	if companyID > 0 || strings.TrimSpace(companyName) != "" {
		brief = s.lookupCompanyProfile(companyID, companyName, position)
	}
	if brief != "" {
		return brief
	}
	return strings.TrimSpace(clientInfo)
}

// jobPositionFinder は職種名から募集職種を引く面。CompanyBriefReader に足さず、
// 実装（CompanyRepository）が持っていれば使う任意の面にして、既存の注入先を壊さない。
type jobPositionFinder interface {
	FindJobPositionByCompanyAndTitle(companyID uint, title string) (*models.CompanyJobPosition, error)
}

// findJobPosition は応募職種名に一致する募集職種を返す。一致しない・引けないときは nil。
func (s *InterviewService) findJobPosition(companyID uint, position string) *models.CompanyJobPosition {
	position = strings.TrimSpace(position)
	if companyID == 0 || position == "" || s.companyRepo == nil {
		return nil
	}
	finder, ok := s.companyRepo.(jobPositionFinder)
	if !ok {
		// 実装が職種を引けないと、職種別の人物像・求人要件が黙って使われなくなる
		log.Printf("[Interview] company repo does not implement jobPositionFinder: %T", s.companyRepo)
		return nil
	}
	job, err := finder.FindJobPositionByCompanyAndTitle(companyID, position)
	if err != nil || job == nil {
		return nil
	}
	return job
}

// lookupCompanyProfile は共有キャッシュ（companies）から企業スナップショットを読む。
// Search / LLM による企業調査は行わない。未登録・未整備なら空文字。
func (s *InterviewService) lookupCompanyProfile(companyID uint, companyName, position string) string {
	comp := s.findCompany(companyID, companyName)
	if comp == nil {
		return ""
	}
	var profile *models.CompanyWeightProfile
	if s.companyRepo != nil {
		if p, err := s.companyRepo.GetWeightProfile(comp.ID, nil); err == nil {
			profile = p
		}
	}
	return company.BuildCompanyBriefForJob(comp, profile, s.findJobPosition(comp.ID, position))
}

// selectInterviewAxes は企業×職種の人物像から質問の軸を選ぶ（#2274）。
// 職種別プロファイルがあればそれを、無ければ企業単位を使う。DB 登録済みで brief に出してよい
// 企業だけが対象で、未審査・ゲスト企業・未登録企業は軸なし（従来の標準トピック）。
func (s *InterviewService) selectInterviewAxes(companyID uint, position string) []interviewAxis {
	if companyID == 0 || s.companyRepo == nil {
		return nil
	}
	comp := s.findCompany(companyID, "")
	if !company.IsBriefVisible(comp) {
		return nil
	}
	var profile *models.CompanyWeightProfile
	if job := s.findJobPosition(comp.ID, position); job != nil {
		if p, err := s.companyRepo.GetWeightProfile(comp.ID, &job.ID); err == nil {
			profile = p
		}
	}
	if profile == nil {
		if p, err := s.companyRepo.GetWeightProfile(comp.ID, nil); err == nil {
			profile = p
		}
	}
	return SelectTargetAxes(profile)
}

func (s *InterviewService) findCompany(companyID uint, companyName string) *models.Company {
	if s.companyRepo == nil {
		return nil
	}
	if companyID > 0 {
		if c, err := s.companyRepo.FindByID(companyID); err == nil && c != nil {
			return c
		}
	}
	name := strings.TrimSpace(companyName)
	if name == "" {
		return nil
	}
	c, err := s.companyRepo.FindByName(name)
	if err != nil || c == nil {
		return nil
	}
	return c
}

// resolveCompanyID は company_id=0（WEB検索・手入力）でも企業名が DB と一致すれば ID を解決する。
// 解決できない場合は 0 のまま（カスタム質問・深掘りは無効）。
func (s *InterviewService) resolveCompanyID(companyID uint, companyName string) uint {
	if company := s.findCompany(companyID, companyName); company != nil {
		return company.ID
	}
	return 0
}

// sttHintCompany は音声認識の補助語に使える企業名・読みを DB から返す。
//
// 解決できなければ両方空にする。補助語は「御社」＋ companyInfo の技術用語だけになり、
// #1603 で測った固有名詞の改善は DB 登録済みの企業で保たれる。
// クライアント直値を補助語へ入れないのが目的なので、ここで LLM 補完
// （lookupCompanyReading）は使わない。STT はターンの先頭にあり、
// ここに LLM 呼び出しを足すと応答遅延がそのまま増える。
func (s *InterviewService) sttHintCompany(companyID uint, companyName string) (string, string) {
	c := s.findCompany(companyID, companyName)
	if c == nil {
		return "", ""
	}
	return strings.TrimSpace(c.Name), strings.TrimSpace(c.NameReading)
}

// resolveCompanyReading は共有DBの NameReading を優先し、無ければモデル知識で補完する。
func (s *InterviewService) resolveCompanyReading(ctx context.Context, companyID uint, companyName string) string {
	if company := s.findCompany(companyID, companyName); company != nil {
		if reading := strings.TrimSpace(company.NameReading); reading != "" {
			return reading
		}
	}
	cacheKey := strings.TrimSpace(companyName)
	if cacheKey == "" {
		return ""
	}
	return s.cachedCompanyReading(ctx, cacheKey, companyName, s.lookupCompanyReading)
}

func (s *InterviewService) cachedCompanyReading(
	ctx context.Context,
	cacheKey string,
	companyName string,
	lookupReading func(context.Context, string) (string, error),
) string {
	if cached, ok := s.companyReadingCache.load(cacheKey); ok {
		return cached
	}

	lookup := func() (any, error) {
		if cached, ok := s.companyReadingCache.load(cacheKey); ok {
			return cached, nil
		}
		reading, err := lookupReading(ctx, companyName)
		if err != nil {
			return "", err
		}
		s.companyReadingCache.store(cacheKey, reading)
		return strings.TrimSpace(reading), nil
	}

	value, err, shared := s.companyReadingFlight.Do(cacheKey, lookup)
	if errors.Is(err, context.Canceled) && shared && ctx.Err() == nil {
		// 共有元のリクエストがキャンセルされた場合だけ、待機側の有効なctxで再試行する。
		// 再びsingleflightを通し、複数の待機リクエストが一斉に外部APIを呼ばないようにする。
		retryCh := s.companyReadingFlight.DoChan(cacheKey, lookup)
		select {
		case retryResult := <-retryCh:
			value, err = retryResult.Val, retryResult.Err
		case <-ctx.Done():
			return ""
		}
	}
	if err != nil {
		return ""
	}
	reading, _ := value.(string)
	return strings.TrimSpace(reading)
}
