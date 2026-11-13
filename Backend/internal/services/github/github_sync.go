package github

import (
	"Backend/internal/safego"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// TriggerAsyncSync 非同期でGitHubデータ同期を開始する（ノンブロッキング）
// force=true でキャッシュを無視して強制同期する
func (s *GitHubService) TriggerAsyncSync(userID uint, force bool) {
	safego.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.SyncUserData(ctx, userID, force); err != nil {
			log.Printf("[GitHubService] async sync failed for user %d: %v", userID, err)
		}
	})
}

// SyncUserData GitHubからリポジトリ・言語比率・コントリビューション数を取得してDBに保存する
// force=true でキャッシュを無視して強制同期する
func (s *GitHubService) SyncUserData(ctx context.Context, userID uint, force bool) error {
	profile, err := s.githubRepo.GetProfile(userID)
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}
	if profile == nil {
		return fmt.Errorf("github profile not found for user %d", userID)
	}

	// キャッシュチェック: 1時間以内に同期済みならスキップ（強制同期時はスキップしない）
	if !force && profile.SyncedAt != nil && time.Since(*profile.SyncedAt) < syncCacheDuration {
		log.Printf("[GitHubService] user %d: skipping sync (last synced %s ago)", userID, time.Since(*profile.SyncedAt).Round(time.Minute))
		return nil
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// トークンを復号する（暗号化されている場合）（#326）
	token, err := loadAccessToken(userID, profile.AccessToken)
	if err != nil {
		return err
	}
	login := profile.GitHubLogin

	// 1. リポジトリ一覧取得（自分のリポジトリ + 所属組織のリポジトリ）
	repos, err := s.fetchRepositories(ctx, client, token)
	if err != nil {
		var scopeErr *InsufficientScopesError
		if errors.As(err, &scopeErr) {
			return scopeErr
		}
		// 無効・失効トークンは再連携を促す
		if isGitHubAuthError(err) {
			return &GitHubReauthRequiredError{Reason: reauthMessageTokenInvalid}
		}
		return err
	}

	// 2. 言語使用比率集計
	langStats := AggregateLanguages(userID, repos)

	// userIDをセット
	for i := range repos {
		repos[i].UserID = userID
	}

	// 3. コントリビューション数取得（GraphQL）
	contributions, err := s.FetchTotalContributions(ctx, client, token, login)
	if err != nil {
		log.Printf("[GitHubService] fetch contributions warning: %v", err)
		// コントリビューション取得失敗はwarn扱いで続行
	}

	// 4. プロフィール統計更新
	now := time.Now()
	profile.TotalContributions = contributions
	profile.PublicRepos = len(repos)
	profile.SyncedAt = &now
	if err := s.githubRepo.UpsertProfile(profile); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	// 5. リポジトリ保存
	if err := s.githubRepo.ReplaceRepositories(userID, repos); err != nil {
		return fmt.Errorf("save repositories: %w", err)
	}

	// 6. 言語比率保存
	if err := s.githubRepo.ReplaceLanguageStats(userID, langStats); err != nil {
		return fmt.Errorf("save language stats: %w", err)
	}

	// 7. スキルスコア算出・保存
	if s.skillScoreService != nil {
		if err := s.skillScoreService.CalculateAndSave(userID, langStats, repos, contributions); err != nil {
			log.Printf("[GitHubService] skill score calculation warning: %v", err)
		}
	}

	log.Printf("[GitHubService] user %d: sync completed (%d repos, %d contributions)", userID, len(repos), contributions)
	return nil
}

// isGitHubNotFoundError は、GitHub API が 404 を返したかを、doGet が返す GitHubAPIError のステータスで判定する。
// README の取得では、404 は「README が無い」という正常な結果として扱う（#2226）。
func isGitHubNotFoundError(err error) bool {
	return githubAPIStatus(err) == http.StatusNotFound
}

// isGitHubAuthError は、トークンが無効・失効している（GitHub API が 401 を返した）かを判定する。
// 以前は「bad credentials」「requires authentication」など 401 の本文の文言も見ていたが、
// どちらも GitHub が 401 で返す本文なので、ステータスだけで同じ範囲を判定できる。
// 復号の失敗は loadAccessToken が GitHubReauthRequiredError で返すので、ここでは見ない。
func isGitHubAuthError(err error) bool {
	return githubAPIStatus(err) == http.StatusUnauthorized
}

// isGitHubForbiddenError は、レート制限以外の 403（組織の SAML SSO が未承認、リポジトリの権限が無い、など）かを判定する。
// レート制限の 403 は CheckRateLimit が rateLimitError で返し、GitHubAPIError にはならない。
func isGitHubForbiddenError(err error) bool {
	return githubAPIStatus(err) == http.StatusForbidden
}
