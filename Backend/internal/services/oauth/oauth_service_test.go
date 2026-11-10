package oauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/config"

	"golang.org/x/oauth2"
)

// #2005: OAuth の外部プロバイダ（Google / GitHub）の応答をフェイクにして、ログインの入口を確かめる。
// 実際のプロバイダへはリクエストを飛ばさない（未登録の URL へのリクエストはテストを失敗させる）。

const (
	fakeTokenURL      = "https://oauth.fake.test/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	githubUserURL     = "https://api.github.com/user"
	githubEmailsURL   = "https://api.github.com/user/emails"
)

type fakeResponse struct {
	status int
	body   string
}

// fakeProvider は URL ごとに決めた応答を返す RoundTripper。未登録の URL は外部へ出さずに失敗させる。
type fakeProvider struct {
	t         *testing.T
	responses map[string]fakeResponse
	mu        sync.Mutex
	requested []string
}

func (f *fakeProvider) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.Scheme + "://" + req.URL.Host + req.URL.Path
	f.mu.Lock()
	f.requested = append(f.requested, u)
	f.mu.Unlock()
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

func TestHandleGoogleCallback(t *testing.T) {
	cases := []struct {
		name        string
		responses   map[string]fakeResponse
		repo        *oauthUserRepoStub
		tenant      uint
		wantErr     string
		wantCreated bool
		wantAdmin   bool
	}{
		{
			name: "新規ユーザーを作る",
			responses: map[string]fakeResponse{
				fakeTokenURL:      okToken,
				googleUserInfoURL: {200, `{"id":"g-1","email":"new@example.com","verified_email":true,"name":"新規","picture":"https://img/p.png"}`},
			},
			repo:        &oauthUserRepoStub{},
			tenant:      7,
			wantCreated: true,
		},
		{
			name: "メール未確認は拒否する",
			responses: map[string]fakeResponse{
				fakeTokenURL:      okToken,
				googleUserInfoURL: {200, `{"id":"g-1","email":"new@example.com","verified_email":false}`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "email not verified",
		},
		{
			name: "トークン交換の失敗はエラー",
			responses: map[string]fakeResponse{
				fakeTokenURL: {400, `{"error":"invalid_grant"}`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "failed to exchange token",
		},
		{
			name: "ユーザー情報が JSON でない",
			responses: map[string]fakeResponse{
				fakeTokenURL:      okToken,
				googleUserInfoURL: {200, `<html>not json</html>`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "failed to unmarshal user info",
		},
		{
			// エラー応答は verified_email を持たないので、未確認として拒否される（ユーザーを作らない）
			name: "ユーザー情報のエラー応答ではユーザーを作らない",
			responses: map[string]fakeResponse{
				fakeTokenURL:      okToken,
				googleUserInfoURL: {401, `{"error":{"code":401,"message":"Invalid Credentials"}}`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "email not verified",
		},
		{
			name: "別テナントの既存ユーザーには紐付けない",
			responses: map[string]fakeResponse{
				fakeTokenURL:      okToken,
				googleUserInfoURL: {200, `{"id":"g-1","email":"other@example.com","verified_email":true}`},
			},
			repo:    &oauthUserRepoStub{byEmail: map[string]*entity.User{"other@example.com": {ID: 5, Email: "other@example.com", OrganizationID: 2}}},
			tenant:  7,
			wantErr: "tenant mismatch",
		},
		{
			name: "既存の管理者には管理者トークンも出す",
			responses: map[string]fakeResponse{
				fakeTokenURL:      okToken,
				googleUserInfoURL: {200, `{"id":"g-9","email":"admin@example.com","verified_email":true}`},
			},
			repo:      &oauthUserRepoStub{byOAuth: map[string]*entity.User{"google:g-9": {ID: 9, Email: "admin@example.com", IsAdmin: true}}},
			wantAdmin: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("USER_SECRET", "user-secret")
			t.Setenv("ADMIN_SECRET", "admin-secret")
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
			if resp.UserToken == "" || resp.OAuthProvider != "google" {
				t.Errorf("resp = %+v", resp)
			}
			if tc.wantCreated {
				if len(tc.repo.created) != 1 {
					t.Fatalf("ユーザーが作られていない")
				}
				u := tc.repo.created[0]
				if u.OAuthProvider != "google" || u.OAuthID != "g-1" || u.OrganizationID != tc.tenant || u.IsAdmin {
					t.Errorf("作ったユーザー = %+v", u)
				}
			}
			if gotAdmin := resp.Token != ""; gotAdmin != tc.wantAdmin {
				t.Errorf("管理者トークン: got %v, want %v", gotAdmin, tc.wantAdmin)
			}
		})
	}
}

func TestHandleGitHubCallback(t *testing.T) {
	cases := []struct {
		name      string
		responses map[string]fakeResponse
		repo      *oauthUserRepoStub
		tenant    uint
		wantErr   string
		wantEmail string
		wantName  string
	}{
		{
			name: "公開メールで新規ユーザーを作り、名前が無ければ login を使う",
			responses: map[string]fakeResponse{
				fakeTokenURL:  okToken,
				githubUserURL: {200, `{"id":42,"login":"octo","name":"","email":"octo@example.com"}`},
			},
			repo:      &oauthUserRepoStub{},
			wantEmail: "octo@example.com",
			wantName:  "octo",
		},
		{
			name: "非公開メールは、確認済みのプライマリを使う",
			responses: map[string]fakeResponse{
				fakeTokenURL:    okToken,
				githubUserURL:   {200, `{"id":42,"login":"octo","name":"Octo"}`},
				githubEmailsURL: {200, `[{"email":"old@example.com","primary":false,"verified":true},{"email":"main@example.com","primary":true,"verified":true}]`},
			},
			repo:      &oauthUserRepoStub{},
			wantEmail: "main@example.com",
			wantName:  "Octo",
		},
		{
			name: "未確認のプライマリは使わず、確認済みのメールを使う",
			responses: map[string]fakeResponse{
				fakeTokenURL:    okToken,
				githubUserURL:   {200, `{"id":42,"login":"octo"}`},
				githubEmailsURL: {200, `[{"email":"unverified@example.com","primary":true,"verified":false},{"email":"ok@example.com","primary":false,"verified":true}]`},
			},
			repo:      &oauthUserRepoStub{},
			wantEmail: "ok@example.com",
			wantName:  "octo",
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
			name: "メール一覧のエラー応答は拒否する",
			responses: map[string]fakeResponse{
				fakeTokenURL:    okToken,
				githubUserURL:   {200, `{"id":42,"login":"octo"}`},
				githubEmailsURL: {401, `{"message":"Bad credentials"}`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "failed to get email",
		},
		{
			name: "トークン交換の失敗はエラー",
			responses: map[string]fakeResponse{
				fakeTokenURL: {401, `{"error":"bad_verification_code"}`},
			},
			repo:    &oauthUserRepoStub{},
			wantErr: "failed to exchange token",
		},
		{
			name: "別テナントの既存の OAuth ユーザーはログインさせない",
			responses: map[string]fakeResponse{
				fakeTokenURL:  okToken,
				githubUserURL: {200, `{"id":42,"login":"octo","email":"octo@example.com"}`},
			},
			repo:    &oauthUserRepoStub{byOAuth: map[string]*entity.User{"github:42": {ID: 3, Email: "octo@example.com", OrganizationID: 2}}},
			tenant:  7,
			wantErr: "tenant mismatch",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("USER_SECRET", "user-secret")
			t.Setenv("ADMIN_SECRET", "admin-secret")
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
			if len(tc.repo.created) != 1 {
				t.Fatalf("ユーザーが作られていない")
			}
			u := tc.repo.created[0]
			if u.Email != tc.wantEmail || u.Name != tc.wantName || u.OAuthProvider != "github" || u.OAuthID != "42" {
				t.Errorf("作ったユーザー = %+v", u)
			}
			if resp.UserToken == "" || resp.Token != "" {
				t.Errorf("一般ユーザーにはユーザートークンだけを出す: %+v", resp)
			}
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
