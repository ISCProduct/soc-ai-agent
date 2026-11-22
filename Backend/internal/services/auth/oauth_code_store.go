package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// OAuthCodeTTL は OAuth 完了時に発行するコードの有効期間。FE は受け取ってすぐ交換する。
const OAuthCodeTTL = 60 * time.Second

var (
	// ErrOAuthCodeInvalid はコードが存在しない・使用済み・期限切れのとき。
	ErrOAuthCodeInvalid = errors.New("oauth code is invalid, used or expired")
	// ErrOAuthBindMismatch はログインを始めたブラウザの照合値と合わないとき。
	ErrOAuthBindMismatch = errors.New("oauth code bind mismatch")
)

// OAuthCodeStore は OAuth 完了時に発行する一度きりの短命コードを保管する (SOCAIAGENT-683)。
// URL にトークンを載せないため、コールバックはコードだけを FE へ渡し、FE がサーバー間で交換する。
// コードには、ログインを始めたブラウザの照合値（verifier の SHA-256）を結びつける。
// 他人が自分のコードを踏ませても、踏んだブラウザの verifier と合わないので交換できない。
//
// ponytail: Redis が無いとメモリに置くため、タスクが複数あると別タスクで交換に失敗する。
// 複数タスクで動かす環境では REDIS_URL を設定する。
type OAuthCodeStore struct {
	rdb *redis.Client
	now func() time.Time

	mu  sync.Mutex
	mem map[string]oauthCodeEntry
}

type oauthCodeEntry struct {
	Payload   []byte    `json:"payload"`
	BindHash  string    `json:"bind_hash"`
	ExpiresAt time.Time `json:"-"`
}

// NewOAuthCodeStore は rdb が nil ならメモリに保管する。
func NewOAuthCodeStore(rdb *redis.Client) *OAuthCodeStore {
	return &OAuthCodeStore{rdb: rdb, now: time.Now, mem: map[string]oauthCodeEntry{}}
}

// Issue は payload を保管し、推測困難なコードを返す。bindHash は base64url(SHA-256(verifier))。
func (s *OAuthCodeStore) Issue(ctx context.Context, payload []byte, bindHash string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(b)
	entry := oauthCodeEntry{Payload: payload, BindHash: bindHash}

	if s.rdb != nil {
		raw, err := json.Marshal(entry)
		if err != nil {
			return "", err
		}
		if err := s.rdb.Set(ctx, codeKey(code), raw, OAuthCodeTTL).Err(); err != nil {
			return "", err
		}
		return code, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, e := range s.mem {
		if now.After(e.ExpiresAt) {
			delete(s.mem, k)
		}
	}
	entry.ExpiresAt = now.Add(OAuthCodeTTL)
	s.mem[codeKey(code)] = entry
	return code, nil
}

// Redeem はコードを消費して payload を返す。照合値が違っても消費するので、総当たりは1回で終わる。
func (s *OAuthCodeStore) Redeem(ctx context.Context, code, verifier string) ([]byte, error) {
	entry, err := s.take(ctx, code)
	if err != nil {
		return nil, err
	}
	if entry.BindHash == "" || subtle.ConstantTimeCompare([]byte(entry.BindHash), []byte(OAuthBindHash(verifier))) != 1 {
		return nil, ErrOAuthBindMismatch
	}
	return entry.Payload, nil
}

func (s *OAuthCodeStore) take(ctx context.Context, code string) (oauthCodeEntry, error) {
	var entry oauthCodeEntry
	if s.rdb != nil {
		raw, err := s.rdb.GetDel(ctx, codeKey(code)).Bytes()
		if errors.Is(err, redis.Nil) {
			return entry, ErrOAuthCodeInvalid
		}
		if err != nil {
			return entry, err
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return entry, ErrOAuthCodeInvalid
		}
		return entry, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.mem[codeKey(code)]
	delete(s.mem, codeKey(code))
	if !ok || s.now().After(entry.ExpiresAt) {
		return oauthCodeEntry{}, ErrOAuthCodeInvalid
	}
	return entry, nil
}

// OAuthBindHash は verifier から照合値（base64url の SHA-256、パディングなし）を作る。
func OAuthBindHash(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// codeKey はコードそのものを保管キーにしない（Redis の中身を見てもコードが分からない）。
func codeKey(code string) string {
	sum := sha256.Sum256([]byte(code))
	return "oauth_code:" + hex.EncodeToString(sum[:])
}
