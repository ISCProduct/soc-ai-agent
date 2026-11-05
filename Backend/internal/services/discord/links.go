package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	defaultLinksParameterName = "/soc-app/discord-backlog-links"
	defaultBacklogBaseURL     = "https://iwasaki-ac.backlog.com"
	defaultBacklogProjectKey  = "SOCAIAGENT"

	// SSM の Standard パラメータは 4KB まで。超える手前で止めて、黙って切れるのを防ぐ。
	// 1人 80 バイト前後なので 40 人強までは入る。
	linksMaxBytes = 4000
)

// Link は Discord ユーザーと Backlog ユーザーの対応。
// 名前は表示用で、突き合わせには BacklogID を使う（Backlog 側の改名に強くするため）。
type Link struct {
	BacklogID int    `json:"id"`
	Name      string `json:"name"`
}

// LinkService は Discord ユーザーID → Backlog ユーザーの対応表を SSM に持つ。
//
// ponytail: 1パラメータに JSON で全員分を入れる。管理者だけが書く運用なので
// 競合はプロセス内 mutex で足りる。40人を超える、または複数人が同時に書くなら
// DynamoDB に移す。
type LinkService struct {
	client    ssmAPI
	paramName string
	mu        sync.Mutex
}

// NewLinkServiceFromEnv は UptimeService と同じ資格情報の取り方で作る。
func NewLinkServiceFromEnv(ctx context.Context) (*LinkService, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	name := os.Getenv("DISCORD_LINKS_SSM_PARAMETER")
	if name == "" {
		name = defaultLinksParameterName
	}
	return &LinkService{client: ssm.NewFromConfig(cfg), paramName: name}, nil
}

func (s *LinkService) List(ctx context.Context) (map[string]Link, error) {
	out, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(s.paramName)})
	if err != nil {
		var notFound *ssmtypes.ParameterNotFound
		if errors.As(err, &notFound) {
			return map[string]Link{}, nil
		}
		return nil, fmt.Errorf("SSM parameter取得に失敗しました: %w", err)
	}
	links := map[string]Link{}
	if v := strings.TrimSpace(aws.ToString(out.Parameter.Value)); v != "" {
		if err := json.Unmarshal([]byte(v), &links); err != nil {
			// 壊れた表を空として上書きすると全員分が消える。止めて気づけるようにする。
			return nil, fmt.Errorf("対応表の形式が不正です: %w", err)
		}
	}
	return links, nil
}

// Set は discordID を Backlog ユーザーに対応づける。
// 同じ Backlog ユーザーが別の Discord ユーザーに紐づいていると、通知が二重に飛んだり
// 本人確認がどちらか不定になるので拒否する（解除してから登録し直す）。
func (s *LinkService) Set(ctx context.Context, discordID string, l Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	links, err := s.List(ctx)
	if err != nil {
		return err
	}
	for id, other := range links {
		if other.BacklogID == l.BacklogID && id != discordID {
			return fmt.Errorf("%s さんは既に <@%s> に紐づいています。先に /unlink で解除してください", l.Name, id)
		}
	}
	links[discordID] = l
	return s.save(ctx, links)
}

// Remove は対応を外す。登録が無ければ false。
func (s *LinkService) Remove(ctx context.Context, discordID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	links, err := s.List(ctx)
	if err != nil {
		return false, err
	}
	if _, ok := links[discordID]; !ok {
		return false, nil
	}
	delete(links, discordID)
	return true, s.save(ctx, links)
}

func (s *LinkService) save(ctx context.Context, links map[string]Link) error {
	b, err := json.Marshal(links)
	if err != nil {
		return err
	}
	if len(b) > linksMaxBytes {
		return fmt.Errorf("登録数の上限に達しました")
	}
	_, err = s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.paramName),
		Value:     aws.String(string(b)),
		Type:      ssmtypes.ParameterTypeString,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("SSM parameter更新に失敗しました: %w", err)
	}
	return nil
}

// SortedIDs は表示順を固定するための Discord ID 一覧（Backlog 名の昇順）。
func SortedIDs(links map[string]Link) []string {
	ids := make([]string, 0, len(links))
	for id := range links {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return links[ids[i]].Name < links[ids[j]].Name })
	return ids
}

// BacklogUser は Backlog のプロジェクト参加者。
type BacklogUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// BacklogUserLister はテストで差し替えるための面。
type BacklogUserLister interface {
	Users(ctx context.Context) ([]BacklogUser, error)
}

// BacklogClient は Backlog API（プロジェクト参加者の取得だけ）。
type BacklogClient struct {
	baseURL    string
	apiKey     string
	projectKey string
	http       *http.Client
}

// NewBacklogClientFromEnv は BACKLOG_API_KEY が無ければ nil を返す（機能を無効にする）。
func NewBacklogClientFromEnv() *BacklogClient {
	key := os.Getenv("BACKLOG_API_KEY")
	if key == "" {
		return nil
	}
	c := &BacklogClient{
		baseURL:    os.Getenv("BACKLOG_BASE_URL"),
		apiKey:     key,
		projectKey: os.Getenv("BACKLOG_PROJECT_KEY"),
		http:       &http.Client{Timeout: 2 * time.Second},
	}
	if c.baseURL == "" {
		c.baseURL = defaultBacklogBaseURL
	}
	if c.projectKey == "" {
		c.projectKey = defaultBacklogProjectKey
	}
	return c
}

func (c *BacklogClient) Users(ctx context.Context) ([]BacklogUser, error) {
	var users []BacklogUser
	if err := c.get(ctx, "/projects/"+url.PathEscape(c.projectKey)+"/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// ProjectKey と BaseURL は main から /deadline の設定に渡すための参照。
func (c *BacklogClient) ProjectKey() string { return c.projectKey }
func (c *BacklogClient) BaseURL() string    { return c.baseURL }

// backlogStatusError は Backlog API が 200 以外を返したときのエラー。
// 呼び出し側が 404(課題なし)や 400(受け付けられない値)を言い分けるために型を持つ。
type backlogStatusError struct{ Status int }

func (e *backlogStatusError) Error() string {
	return fmt.Sprintf("Backlog API が %d を返しました", e.Status)
}

// get は Backlog API を GET して out に読み込む。
func (c *BacklogClient) get(ctx context.Context, path string, q url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, q, nil, out)
}

// do は Backlog API を呼ぶ。form があればフォームのボディで送る（PATCH 等）。out が nil なら応答は読まない。
// apiKey はクエリ文字列で渡す（Backlog の仕様）。url.Error にURLごと入るので、
// エラーに丸ごと包むとログにキーが残る。理由だけを取り出す。
func (c *BacklogClient) do(ctx context.Context, method, path string, q, form url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	q.Set("apiKey", c.apiKey)
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v2"+path+"?"+q.Encode(), body)
	if err != nil {
		return errors.New("Backlog API のリクエスト作成に失敗しました")
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("Backlog API の呼び出しに失敗しました: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &backlogStatusError{Status: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("Backlog API の応答を読めませんでした: %w", err)
	}
	return nil
}

// ResolveBacklogUser は名前の完全一致で1人に絞る。
// 部分一致で拾うと、似た名前の別人に紐づける事故になる。
func ResolveBacklogUser(users []BacklogUser, name string) (BacklogUser, error) {
	name = strings.TrimSpace(name)
	var found []BacklogUser
	for _, u := range users {
		if u.Name == name {
			found = append(found, u)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return BacklogUser{}, fmt.Errorf("Backlog に「%s」という名前のメンバーがいません。候補から選んでください", name)
	default:
		return BacklogUser{}, fmt.Errorf("Backlog に「%s」が複数います。管理者に確認してください", name)
	}
}

// SuggestBacklogUsers はオートコンプリートの候補（部分一致・大文字小文字無視、最大25件）。
func SuggestBacklogUsers(users []BacklogUser, typed string) []string {
	typed = strings.ToLower(strings.TrimSpace(typed))
	var out []string
	for _, u := range users {
		if typed == "" || strings.Contains(strings.ToLower(u.Name), typed) {
			out = append(out, u.Name)
			if len(out) == 25 { // Discord の上限
				break
			}
		}
	}
	return out
}
