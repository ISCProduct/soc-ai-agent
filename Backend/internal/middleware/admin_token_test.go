package middleware

import (
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/domain/entity"
)

const testAdminSecret = "test-admin-secret"

// TestParseAdminToken は署名・有効期限・形式の検証をテーブル駆動で確認する（#1155）。
func TestParseAdminToken(t *testing.T) {
	// 実行環境に ADMIN_TOKEN_TTL_HOURS が設定されていると TTL 境界のケースが落ちるため固定する
	t.Setenv("ADMIN_TOKEN_TTL_HOURS", "")
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	const userID = uint(7)
	const email = "admin@example.com"

	valid := generateAdminTokenAt(userID, email, testAdminSecret, now)

	tests := []struct {
		name    string
		token   string
		userID  uint
		email   string
		secret  string
		at      time.Time
		wantErr error
	}{
		{name: "発行直後は有効", token: valid, userID: userID, email: email, secret: testAdminSecret, at: now},
		{
			name: "TTL内なら有効", token: valid, userID: userID, email: email, secret: testAdminSecret,
			at: now.Add(defaultAdminTokenTTL - time.Minute),
		},
		{
			name: "TTLを超えると期限切れ", token: valid, userID: userID, email: email, secret: testAdminSecret,
			at: now.Add(defaultAdminTokenTTL + time.Minute), wantErr: ErrAdminTokenExpired,
		},
		{
			// 旧形式を持っているのは正規の管理者なので、再ログインを案内できるよう期限切れ扱いにする
			name:   "期限を持たない旧形式は期限切れとして拒否（署名が一致する場合）",
			token:  legacyAdminSignature(userID, email, testAdminSecret),
			userID: userID, email: email, secret: testAdminSecret, at: now, wantErr: ErrAdminTokenExpired,
		},
		{
			name:  "旧形式ですらない文字列は無効",
			token: "garbage", userID: userID, email: email, secret: testAdminSecret,
			at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			// 署名を見ずに形式だけで expired を返すと、hex 64文字を投げるだけで
			// 「そのメールアドレスが管理者か」をレスポンスの差から判別できてしまう
			name:  "旧形式の長さでも署名が違えば無効（列挙オラクルを作らない）",
			token: strings.Repeat("ab", 32), userID: userID, email: email, secret: testAdminSecret,
			at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "発行時刻に先行ゼロを付けた別表記は無効",
			token: func() string {
				raw, sig, _ := splitForTest(valid)
				return "0" + raw + "." + sig
			}(),
			userID: userID, email: email, secret: testAdminSecret, at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "署名が改ざんされていると拒否", token: valid[:len(valid)-1] + "0",
			userID: userID, email: email, secret: testAdminSecret, at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "発行時刻を書き換えると署名不一致で拒否",
			token: func() string {
				_, sig, _ := splitForTest(valid)
				return "9999999999." + sig
			}(),
			userID: userID, email: email, secret: testAdminSecret, at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "他ユーザーIDでは拒否", token: valid, userID: 8, email: email, secret: testAdminSecret,
			at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "他メールアドレスでは拒否", token: valid, userID: userID, email: "other@example.com",
			secret: testAdminSecret, at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "別シークレットでは拒否", token: valid, userID: userID, email: email, secret: "other-secret",
			at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "シークレット未設定では拒否", token: valid, userID: userID, email: email, secret: "",
			at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name: "空トークンは拒否", token: "", userID: userID, email: email, secret: testAdminSecret,
			at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name:   "許容範囲を超える未来発行は拒否",
			token:  generateAdminTokenAt(userID, email, testAdminSecret, now.Add(adminTokenClockSkew+time.Minute)),
			userID: userID, email: email, secret: testAdminSecret, at: now, wantErr: ErrAdminTokenInvalid,
		},
		{
			name:   "わずかな時刻ずれは許容",
			token:  generateAdminTokenAt(userID, email, testAdminSecret, now.Add(time.Minute)),
			userID: userID, email: email, secret: testAdminSecret, at: now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAdminTokenAt(tt.token, tt.userID, tt.email, tt.secret, tt.at)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("err=%v, 有効であるべき", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v want %v", err, tt.wantErr)
			}
		})
	}
}

// splitForTest はテスト用にトークンを発行時刻と署名へ分解する。
func splitForTest(token string) (string, string, bool) {
	for i := range len(token) {
		if token[i] == '.' {
			return token[:i], token[i+1:], true
		}
	}
	return token, "", false
}

// TestGenerateAdminTokenIsNotStatic は同じ管理者でも発行ごとに値が変わることを確認する。
// 静的トークンだった旧実装（#1155 の原因）への回帰を防ぐ。
func TestGenerateAdminTokenIsNotStatic(t *testing.T) {
	base := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	first := generateAdminTokenAt(1, "a@example.com", testAdminSecret, base)
	second := generateAdminTokenAt(1, "a@example.com", testAdminSecret, base.Add(time.Second))
	if first == second {
		t.Fatal("発行時刻が違うのに同一トークンが生成された（静的トークンに戻っている）")
	}
}

// TestAdminTokenTTLFromEnv は ADMIN_TOKEN_TTL_HOURS が反映されることを確認する。
func TestAdminTokenTTLFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "未設定なら既定値", value: "", want: defaultAdminTokenTTL},
		{name: "1時間へ短縮", value: "1", want: time.Hour},
		{name: "不正値は既定値", value: "abc", want: defaultAdminTokenTTL},
		{name: "0以下は既定値", value: "0", want: defaultAdminTokenTTL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ADMIN_TOKEN_TTL_HOURS", tt.value)
			if got := adminTokenTTL(); got != tt.want {
				t.Fatalf("adminTokenTTL()=%v want %v", got, tt.want)
			}
		})
	}
}

// TestValidateAdminTokenForUser は管理者1名単位の失効（admin_token_not_before）と、
// 期限切れだけを区別して返すことを確認する。
func TestValidateAdminTokenForUser(t *testing.T) {
	now := time.Now()
	user := &entity.User{ID: 7, Email: "admin@example.com", IsAdmin: true}
	token := generateAdminTokenAt(user.ID, user.Email, testAdminSecret, now)

	if err := ValidateAdminTokenForUser(token, user, testAdminSecret); err != nil {
		t.Fatalf("失効指定が無いのに拒否された: %v", err)
	}

	// 発行より後を失効点にすると拒否される（理由は invalid。失効済みであることを相手に伝えない）
	after := now.Add(time.Minute)
	user.AdminTokenNotBefore = &after
	if err := ValidateAdminTokenForUser(token, user, testAdminSecret); err != ErrAdminTokenInvalid {
		t.Fatalf("err = %v, want ErrAdminTokenInvalid", err)
	}

	// 失効点より後に発行し直したトークンは通る
	reissued := generateAdminTokenAt(user.ID, user.Email, testAdminSecret, after.Add(time.Second))
	if err := ValidateAdminTokenForUser(reissued, user, testAdminSecret); err != nil {
		t.Fatalf("失効後に再発行したトークンが拒否された: %v", err)
	}

	// 他の管理者は影響を受けない
	other := &entity.User{ID: 8, Email: "other@example.com", IsAdmin: true}
	otherToken := generateAdminTokenAt(other.ID, other.Email, testAdminSecret, now)
	if err := ValidateAdminTokenForUser(otherToken, other, testAdminSecret); err != nil {
		t.Fatalf("失効指定のない別管理者のトークンまで無効になっている: %v", err)
	}

	if err := ValidateAdminTokenForUser(token, nil, testAdminSecret); err != ErrAdminTokenInvalid {
		t.Fatalf("user=nil で err = %v, want ErrAdminTokenInvalid", err)
	}
}

// TestValidateAdminTokenForUser_ExpiredIsDistinguishable は期限切れが
// ErrAdminTokenExpired として返ることを確認する。
//
// 呼び出し側（EchoAdminAuth）が「再ログインしてください」と案内できるようにするため、
// 署名が正しいトークンの期限切れだけは無効と区別する。
func TestValidateAdminTokenForUser_ExpiredIsDistinguishable(t *testing.T) {
	user := &entity.User{ID: 7, Email: "admin@example.com", IsAdmin: true}
	old := time.Now().Add(-defaultAdminTokenTTL - time.Hour)
	token := generateAdminTokenAt(user.ID, user.Email, testAdminSecret, old)

	if err := ValidateAdminTokenForUser(token, user, testAdminSecret); err != ErrAdminTokenExpired {
		t.Fatalf("err = %v, want ErrAdminTokenExpired", err)
	}

	// 署名が違う場合は期限切れではなく invalid（期限の情報を与えない）
	tampered := token[:len(token)-1] + "0"
	if err := ValidateAdminTokenForUser(tampered, user, testAdminSecret); err != ErrAdminTokenInvalid {
		t.Fatalf("err = %v, want ErrAdminTokenInvalid", err)
	}
}
