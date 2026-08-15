package github

import (
	"Backend/internal/crypto"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// TriggerAsyncSync 非同期でGitHubデータ同期を開始する（ノンブロッキング）
// force=true でキャッシュを無視して強制同期する
func (s *GitHubService) TriggerAsyncSync(userID uint, force bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.SyncUserData(ctx, userID, force); err != nil {
			log.Printf("[GitHubService] async sync failed for user %d: %v", userID, err)
		}
	}()
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
	token := profile.AccessToken
	if encryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY"); encryptionKey != "" {
		decrypted, err := crypto.DecryptToken(profile.AccessToken, encryptionKey)
		if err != nil {
			log.Printf("[GitHubService] failed to decrypt access token for user %d: %v", userID, err)
			return &GitHubReauthRequiredError{Reason: "GitHubアクセストークンの復号に失敗しました。GitHubアカウントを再連携してください。"}
		}
		token = decrypted
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
			return &GitHubReauthRequiredError{Reason: "GitHubアクセストークンが無効です。GitHubアカウントを再連携してください。"}
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

func isGitHubAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status 401") ||
		strings.Contains(msg, "bad credentials") ||
		strings.Contains(msg, "requires authentication") ||
		strings.Contains(msg, "access token decrypt failed")
}
