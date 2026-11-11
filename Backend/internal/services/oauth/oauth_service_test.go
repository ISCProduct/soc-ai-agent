package oauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/config"
	"Backend/internal/middleware"

	"golang.org/x/oauth2"
)

// #2005: OAuth の外部プロバイダ（Google / GitHub）の応答をフェイクにして、ログインの入口を確かめる。
// 実際のプロバイダへはリクエストを飛ばさない（未登録の URL へのリクエストはテストを失敗させる）。

const (
	fakeTokenURL      = "https://oauth.fake.test/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	githubUserURL     = "https://api.github.com/user"
	githubEmailsURL   = "https://api.github.com/user/emails"

	userSecret  = "user-secret"
	adminSecret = "admin-secret"
)

type fakeResponse struct {
	status int
	body   string
}

// fakeProvider は URL ごとに決めた応答を返す RoundTripper。未登録の URL は外部へ出さずに失敗させる。
type fakeProvider struct {
	t         *testing.T
	responses map[string]fakeResponse
}

func (f *fakeProvider) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.Scheme + "://" + req.URL.Host + req.URL.Path
	r, ok := f.responses[u]
	if !ok {
		f.t.Errorf("想定外の外部リクエスト: %s", u)
		return nil, errors.New("unexpected request: " + u)
	}
	return &http.Response{
		StatusCode: r.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Request:    req,
	}, nil
}

func (f *fakeProvider) ctx() context.Context {
	return context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: f})
}

var okToken = fakeResponse{200, `{"access_token":"fake-access-token","token_type":"bearer"}`}

type oauthUserRepoStub struct {
	repository.UserRepository
	byOAuth map[string]*entity.User
	byEmail map[string]*entity.User
	created []*entity.User
	updated []*entity.User
}

func (r *oauthUserRepoStub) GetUserByOAuth(provider, id string) (*entity.User, error) {
	return r.byOAuth[provider+":"+id], nil
}
func (r *oauthUserRepoStub) GetUserByEmail(email string) (*entity.User, error) {
	return r.byEmail[email], nil
}
func (r *oauthUserRepoStub) CreateUser(u *entity.User) error {
	u.ID = 100
	r.created = append(r.created, u)
	return nil
}
func (r *oauthUserRepoStub) UpdateUser(u *entity.User) error {
	r.updated = append(r.updated, u)
	return nil
}

func newOAuthTestService(repo *oauthUserRepoStub) *OAuthService {
	endpoint := oauth2.Endpoint{AuthURL: "https://oauth.fake.test/auth", TokenURL: fakeTokenURL}
	return NewOAuthService(repo, &config.OAuthConfig{
		Google: &oauth2.Config{ClientID: "id", ClientSecret: "secret", Endpoint: endpoint},
		GitHub: &oauth2.Config{ClientID: "id", ClientSecret: "secret", Endpoint: endpoint},
	}, nil)
}

// wantLogin は成功したときの期待。どのユーザーで、どの経路（新規作成・紐付け・既存）でログインしたか。
type wantLogin struct {
	userID    uint
	email     string
	created   *entity.User // 新規作成したユーザーの期待値（nil なら作成しない）
	linked    *entity.User // メール一致の既存ユーザーへ紐付けたときの更新後の期待値（nil なら更新しない）
	wantAdmin bool         // 管理者トークンも出すか
}

// checkLogin は、ログインしたユーザー、トークンが正しい secret で作られていること、作成・更新した内容を確かめる。
func checkLogin(t *testing.T, provider string, userToken, adminToken string, gotUserID uint, gotProvider string, repo *oauthUserRepoStub, w wantLogin) {
	t.Helper()
	if gotUserID != w.userID || gotProvider != provider {
		t.Errorf("ログインしたユーザー = %d (%s), want %d (%s)", gotUserID, gotProvider, w.userID, provider)
	}
	if !middleware.VerifyUserToken(userToken, w.userID, w.email, userSecret) {
		t.Errorf("ユーザートークンが USER_SECRET で検証できない")
	}
	if w.wantAdmin {
		if _, err := middleware.ParseAdminToken(adminToken, w.userID, w.email, adminSecret); err != nil {
			t.Errorf("管理者トークンが ADMIN_SECRET で検証できない: %v", err)
		}
	} else if adminToken != "" {
		t.Errorf("一般ユーザーに管理者トークンを出した")
	}

	switch {
	case w.created != nil:
		if len(repo.created) != 1 || len(repo.updated) != 0 {
			t.Fatalf("created=%d updated=%d, want 1/0", len(repo.created), len(repo.updated))
		}
		got := repo.created[0]
		want := w.created
		if got.Email != want.Email || got.Name != want.Name || got.OAuthProvider != want.OAuthProvider ||
			got.OAuthID != want.OAuthID || got.AvatarURL != want.AvatarURL || got.OrganizationID != want.OrganizationID ||
			got.IsAdmin || got.IsGuest {
			t.Errorf("作ったユーザー = %+v\nwant %+v", got, want)
		}
	case w.linked != nil:
		if len(repo.created) != 0 || len(repo.updated) != 1 {
			t.Fatalf("created=%d updated=%d, want 0/1", len(repo.created), len(repo.updated))
		}
		got := repo.updated[0]
		want := w.linked
		if got.ID != want.ID || got.OAuthProvider != want.OAuthProvider || got.OAuthID != want.OAuthID ||
			got.AvatarURL != want.AvatarURL || got.Name != want.Name {
			t.Errorf("紐付けたユーザー = %+v\nwant %+v", got, want)
		}
	default:
		if len(repo.created) != 0 || len(repo.updated) != 0 {
			t.Errorf("既存の OAuth ユーザーなのに作成・更新した: created=%d updated=%d", len(repo.created), len(repo.updated))
		}
	}
}

func TestHandleGoogleCallback(t *testing.T) {
	const verified = `{"id":"g-1","email":"new@example.com","verified_email":true,"name":"新規","picture":"https://img/p.png"}`
	cases := []struct {
		name      string
		responses map[string]fakeResponse
		repo      *oauthUserRepoStub
		tenant    uint
		wantErr   string
		want      wantLogin
	}{
		{
			name:      "新規ユーザーを作る（テナントの組織IDを引き継ぐ）",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, verified}},
			repo:      &oauthUserRepoStub{},
			tenant:    7,
			want: wantLogin{userID: 100, email: "new@example.com", created: &entity.User{
				Email: "new@example.com", Name: "新規", OAuthProvider: "google", OAuthID: "g-1",
				AvatarURL: "https://img/p.png", OrganizationID: 7,
			}},
		},
		{
			name:      "メールが一致する既存ユーザーに紐付け、名前は空のときだけ補う",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, verified}},
			repo: &oauthUserRepoStub{byEmail: map[string]*entity.User{
				"new@example.com": {ID: 5, Email: "new@example.com", Name: "", OrganizationID: 7},
			}},
			tenant: 7,
			want: wantLogin{userID: 5, email: "new@example.com", linked: &entity.User{
				ID: 5, OAuthProvider: "google", OAuthID: "g-1", AvatarURL: "https://img/p.png", Name: "新規",
			}},
		},
		{
			name:      "紐付けでは、既存ユーザーの名前を上書きしない",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, verified}},
			repo: &oauthUserRepoStub{byEmail: map[string]*entity.User{
				"new@example.com": {ID: 5, Email: "new@example.com", Name: "元の名前"},
			}},
			want: wantLogin{userID: 5, email: "new@example.com", linked: &entity.User{
				ID: 5, OAuthProvider: "google", OAuthID: "g-1", AvatarURL: "https://img/p.png", Name: "元の名前",
			}},
		},
		{
			name:      "テナント指定なし（0）なら、別組織の既存ユーザーにも紐付ける",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, verified}},
			repo: &oauthUserRepoStub{byEmail: map[string]*entity.User{
				"new@example.com": {ID: 5, Email: "new@example.com", Name: "元の名前", OrganizationID: 2},
			}},
			tenant: 0,
			want: wantLogin{userID: 5, email: "new@example.com", linked: &entity.User{
				ID: 5, OAuthProvider: "google", OAuthID: "g-1", AvatarURL: "https://img/p.png", Name: "元の名前",
			}},
		},
		{
			name:      "管理者は別テナントの既存ユーザーでも紐付ける（テナント制約の例外）",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, verified}},
			repo: &oauthUserRepoStub{byEmail: map[string]*entity.User{
				"new@example.com": {ID: 5, Email: "new@example.com", Name: "管理者", OrganizationID: 2, IsAdmin: true},
			}},
			tenant: 7,
			want: wantLogin{userID: 5, email: "new@example.com", wantAdmin: true, linked: &entity.User{
				ID: 5, OAuthProvider: "google", OAuthID: "g-1", AvatarURL: "https://img/p.png", Name: "管理者",
			}},
		},
		{
			name:      "別テナントの既存ユーザー（非管理者）には紐付けない",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, verified}},
			repo:      &oauthUserRepoStub{byEmail: map[string]*entity.User{"new@example.com": {ID: 5, Email: "new@example.com", OrganizationID: 2}}},
			tenant:    7,
			wantErr:   "tenant mismatch",
		},
		{
			name:      "既存の OAuth ユーザーが別テナント（非管理者）ならログインさせない",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, verified}},
			repo:      &oauthUserRepoStub{byOAuth: map[string]*entity.User{"google:g-1": {ID: 6, Email: "new@example.com", OrganizationID: 2}}},
			tenant:    7,
			wantErr:   "tenant mismatch",
		},
		{
			name:      "既存の管理者には管理者トークンも出す",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, `{"id":"g-9","email":"admin@example.com","verified_email":true}`}},
			repo:      &oauthUserRepoStub{byOAuth: map[string]*entity.User{"google:g-9": {ID: 9, Email: "admin@example.com", IsAdmin: true}}},
			want:      wantLogin{userID: 9, email: "admin@example.com", wantAdmin: true},
		},
		{
			name:      "職員（is_admin=false）にも管理者トークンを出す",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, `{"id":"g-8","email":"staff@example.com","verified_email":true}`}},
			repo:      &oauthUserRepoStub{byOAuth: map[string]*entity.User{"google:g-8": {ID: 8, Email: "staff@example.com", Role: entity.UserRoleStaff}}},
			want:      wantLogin{userID: 8, email: "staff@example.com", wantAdmin: true},
		},
		{
			name:      "メール未確認は拒否する",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, `{"id":"g-1","email":"new@example.com","verified_email":false}`}},
			repo:      &oauthUserRepoStub{},
			wantErr:   "email not verified",
		},
		{
			name:      "トークン交換の失敗はエラー",
			responses: map[string]fakeResponse{fakeTokenURL: {400, `{"error":"invalid_grant"}`}},
			repo:      &oauthUserRepoStub{},
			wantErr:   "failed to exchange token",
		},
		{
			name:      "ユーザー情報が JSON でない",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {200, `<html>not json</html>`}},
			repo:      &oauthUserRepoStub{},
			wantErr:   "failed to unmarshal user info",
		},
		{
			// 本体は HTTP ステータスを見ていない（#2154 で対応予定）。今は、エラー応答の JSON に
			// verified_email が無いので「未確認」として拒否されている、という現状の動きを固定している。
			name:      "401 のエラー応答は verified_email を持たないので未確認として拒否される（ステータスでは拒否していない）",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, googleUserInfoURL: {401, `{"error":{"code":401,"message":"Invalid Credentials"}}`}},
			repo:      &oauthUserRepoStub{},
			wantErr:   "email not verified",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("USER_SECRET", userSecret)
			t.Setenv("ADMIN_SECRET", adminSecret)
			fake := &fakeProvider{t: t, responses: tc.responses}

			resp, err := newOAuthTestService(tc.repo).HandleGoogleCallback(fake.ctx(), "code", tc.tenant)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				if len(tc.repo.created) != 0 || len(tc.repo.updated) != 0 {
					t.Errorf("失敗したのにユーザーを作成・更新した: created=%d updated=%d", len(tc.repo.created), len(tc.repo.updated))
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			checkLogin(t, "google", resp.UserToken, resp.Token, resp.UserID, resp.OAuthProvider, tc.repo, tc.want)
		})
	}
}

func TestHandleGitHubCallback(t *testing.T) {
	const publicEmail = `{"id":42,"login":"octo","name":"","email":"octo@example.com","avatar_url":"https://img/octo.png"}`
	cases := []struct {
		name      string
		responses map[string]fakeResponse
		repo      *oauthUserRepoStub
		tenant    uint
		wantErr   string
		want      wantLogin
	}{
		{
			name:      "公開メールで新規ユーザーを作り、名前が無ければ login を使う",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, githubUserURL: {200, publicEmail}},
			repo:      &oauthUserRepoStub{},
			tenant:    7,
			want: wantLogin{userID: 100, email: "octo@example.com", created: &entity.User{
				Email: "octo@example.com", Name: "octo", OAuthProvider: "github", OAuthID: "42",
				AvatarURL: "https://img/octo.png", OrganizationID: 7,
			}},
		},
		{
			name: "非公開メールは、確認済みのプライマリを使う",
			responses: map[string]fakeResponse{
				fakeTokenURL:    okToken,
				githubUserURL:   {200, `{"id":42,"login":"octo","name":"Octo"}`},
				githubEmailsURL: {200, `[{"email":"old@example.com","primary":false,"verified":true},{"email":"main@example.com","primary":true,"verified":true}]`},
			},
			repo: &oauthUserRepoStub{},
			want: wantLogin{userID: 100, email: "main@example.com", created: &entity.User{
				Email: "main@example.com", Name: "Octo", OAuthProvider: "github", OAuthID: "42",
			}},
		},
		{
			name: "未確認のプライマリは使わず、確認済みのメールを使う",
			responses: map[string]fakeResponse{
				fakeTokenURL:    okToken,
				githubUserURL:   {200, `{"id":42,"login":"octo"}`},
				githubEmailsURL: {200, `[{"email":"unverified@example.com","primary":true,"verified":false},{"email":"ok@example.com","primary":false,"verified":true}]`},
			},
			repo: &oauthUserRepoStub{},
			want: wantLogin{userID: 100, email: "ok@example.com", created: &entity.User{
				Email: "ok@example.com", Name: "octo", OAuthProvider: "github", OAuthID: "42",
			}},
		},
		{
			name:      "メールが一致する既存ユーザーに紐付け、名前が空なら name → login の順で補う",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, githubUserURL: {200, publicEmail}},
			repo: &oauthUserRepoStub{byEmail: map[string]*entity.User{
				"octo@example.com": {ID: 5, Email: "octo@example.com", Name: "", OrganizationID: 7},
			}},
			tenant: 7,
			want: wantLogin{userID: 5, email: "octo@example.com", linked: &entity.User{
				ID: 5, OAuthProvider: "github", OAuthID: "42", AvatarURL: "https://img/octo.png", Name: "octo",
			}},
		},
		{
			name:      "管理者は別テナントの既存ユーザーでも紐付ける（テナント制約の例外）",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, githubUserURL: {200, publicEmail}},
			repo: &oauthUserRepoStub{byEmail: map[string]*entity.User{
				"octo@example.com": {ID: 5, Email: "octo@example.com", Name: "管理者", OrganizationID: 2, IsAdmin: true},
			}},
			tenant: 7,
			want: wantLogin{userID: 5, email: "octo@example.com", wantAdmin: true, linked: &entity.User{
				ID: 5, OAuthProvider: "github", OAuthID: "42", AvatarURL: "https://img/octo.png", Name: "管理者",
			}},
		},
		{
			name:      "既存の管理者の OAuth ユーザーは別テナントでもログインできる",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, githubUserURL: {200, publicEmail}},
			repo:      &oauthUserRepoStub{byOAuth: map[string]*entity.User{"github:42": {ID: 3, Email: "octo@example.com", OrganizationID: 2, IsAdmin: true}}},
			tenant:    7,
			want:      wantLogin{userID: 3, email: "octo@example.com", wantAdmin: true},
		},
		{
			name:      "別テナントの既存の OAuth ユーザー（非管理者）はログインさせない",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, githubUserURL: {200, publicEmail}},
			repo:      &oauthUserRepoStub{byOAuth: map[string]*entity.User{"github:42": {ID: 3, Email: "octo@example.com", OrganizationID: 2}}},
			tenant:    7,
			wantErr:   "tenant mismatch",
		},
		{
			name:      "別テナントの既存ユーザー（非管理者）にはメールで紐付けない",
			responses: map[string]fakeResponse{fakeTokenURL: okToken, githubUserURL: {200, publicEmail}},
			repo:      &oauthUserRepoStub{byEmail: map[string]*entity.User{"octo@example.com": {ID: 5, Email: "octo@example.com", OrganizationID: 2}}},
			tenant:    7,
			wantErr:   "tenant mismatch",
		},
		{
			name: "確認済みのメールが無ければ拒否する",
			responses: map[string]fakeResponse{
				fakeTokenURL:    okToken,
				githubUserURL:   {200, `{"id":42,"login":"octo"}`},
				githubEmailsURL: {200, `[{"email":"unverified@example.com","primary":true,"verified":false}]`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "no verified email found",
		},
		{
			// 本体は HTTP ステータスを見ていない（#2154 で対応予定）。今は、エラー応答の JSON が
			// 配列ではないので型の不一致で拒否されている、という現状の動きを固定している。
			name: "メール一覧の 401 応答は配列ではないので型の不一致で拒否される（ステータスでは拒否していない）",
			responses: map[string]fakeResponse{
				fakeTokenURL:    okToken,
				githubUserURL:   {200, `{"id":42,"login":"octo"}`},
				githubEmailsURL: {401, `{"message":"Bad credentials"}`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "failed to get email",
		},
		{
			name:      "トークン交換の失敗はエラー",
			responses: map[string]fakeResponse{fakeTokenURL: {401, `{"error":"bad_verification_code"}`}},
			repo:      &oauthUserRepoStub{},
			wantErr:   "failed to exchange token",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("USER_SECRET", userSecret)
			t.Setenv("ADMIN_SECRET", adminSecret)
			fake := &fakeProvider{t: t, responses: tc.responses}

			resp, err := newOAuthTestService(tc.repo).HandleGitHubCallback(fake.ctx(), "code", tc.tenant)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				if len(tc.repo.created) != 0 || len(tc.repo.updated) != 0 {
					t.Errorf("失敗したのにユーザーを作成・更新した")
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			checkLogin(t, "github", resp.UserToken, resp.Token, resp.UserID, resp.OAuthProvider, tc.repo, tc.want)
		})
	}
}

func TestAuthURLs_CarryState(t *testing.T) {
	svc := newOAuthTestService(&oauthUserRepoStub{})
	for name, u := range map[string]string{
		"google": svc.GetGoogleAuthURL("state-123"),
		"github": svc.GetGitHubAuthURL("state-123"),
	} {
		if !strings.Contains(u, "state=state-123") {
			t.Errorf("%s: 認可 URL に state が無い: %s", name, u)
		}
	}
}
