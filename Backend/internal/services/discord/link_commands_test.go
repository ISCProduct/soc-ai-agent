package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// memSSM は書いた値を次の読み出しで返す SSM。fakeSSM は書き込みを覚えないため、
// 登録→一覧→解除のように続けて呼ぶテストには使えない。
type memSSM struct {
	value  *string
	putErr error
}

func (m *memSSM) GetParameter(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.value == nil {
		return nil, &ssmtypes.ParameterNotFound{}
	}
	return &ssm.GetParameterOutput{Parameter: &ssmtypes.Parameter{Value: m.value}}, nil
}

func (m *memSSM) PutParameter(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	m.value = in.Value
	return &ssm.PutParameterOutput{}, nil
}

type backlogStub struct {
	users []BacklogUser
	err   error
}

func (b backlogStub) Users(context.Context) ([]BacklogUser, error) { return b.users, b.err }

var testUsers = []BacklogUser{{ID: 1, Name: "大橋和幸"}, {ID: 2, Name: "山田太郎"}, {ID: 3, Name: "山田花子"}}

func newLinkHandler(mem *memSSM, b BacklogUserLister) *Handler {
	return NewHandler(nil, nil, noopDispatcher(), "", "role-allowed").
		WithLinks(&LinkService{client: mem, paramName: "/p/links"}, b)
}

func linkCmd(name string, roles []string, opts ...CommandOption) *Interaction {
	return &Interaction{
		Type:   InteractionTypeApplicationCommand,
		Member: &Member{Roles: roles, User: &User{ID: "admin", Username: "admin"}},
		Data:   &InteractionData{Name: name, Options: opts},
	}
}

func opt(name, v string) CommandOption { return CommandOption{Name: name, Value: v} }

func TestLink_登録して一覧と解除ができる(t *testing.T) {
	mem := &memSSM{}
	h := newLinkHandler(mem, backlogStub{users: testUsers})
	admin := []string{"role-allowed"}

	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin, opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸"))))
	if !strings.Contains(got, "✅") {
		t.Fatalf("登録に失敗: %s", got)
	}

	got = contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLinkList, admin)))
	if !strings.Contains(got, "<@111>") || !strings.Contains(got, "大橋和幸") {
		t.Fatalf("一覧に出ない: %s", got)
	}

	got = contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameUnlink, admin, opt(OptionNameMember, "111"))))
	if !strings.Contains(got, "解除") {
		t.Fatalf("解除に失敗: %s", got)
	}
	got = contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameUnlink, admin, opt(OptionNameMember, "111"))))
	if !strings.Contains(got, "登録されていません") {
		t.Fatalf("未登録の解除が成功扱い: %s", got)
	}
}

func TestLinkListPages_100人でも1通が2000文字を超えない(t *testing.T) {
	links := map[string]Link{}
	for i := 0; i < 100; i++ {
		links[fmt.Sprintf("%018d", i)] = Link{BacklogID: i + 1, Name: "名前"}
	}
	pages := linkListPages(links)
	if len(pages) < 2 {
		t.Fatalf("100人を1通にまとめた: %dページ", len(pages))
	}
	seen := 0
	for i, page := range pages {
		if n := utf8.RuneCountInString(page); n > discordMessageLimit {
			t.Fatalf("%dページが %d文字", i+1, n)
		}
		seen += strings.Count(page, "\n・")
	}
	if seen != 100 {
		t.Fatalf("載った人数 = %d", seen)
	}
	if !strings.Contains(pages[0], "1/"+fmt.Sprint(len(pages))+"ページ") {
		t.Fatalf("ページ番号が無い: %s", pages[0])
	}
}

func TestLink_GitHubログインを覚え再登録でも残す(t *testing.T) {
	mem := &memSSM{}
	h := newLinkHandler(mem, backlogStub{users: testUsers})
	admin := []string{"role-allowed"}
	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin,
		opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸"), opt(OptionNameGitHub, "@octocat"))))
	if !strings.Contains(got, "octocat") {
		t.Fatalf("登録メッセージ: %s", got)
	}
	if mem.value == nil || !strings.Contains(*mem.value, `"github":"octocat"`) {
		t.Fatalf("保存されていない: %v", mem.value)
	}
	got = contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin,
		opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸"))))
	if !strings.Contains(got, "octocat") {
		t.Fatalf("引数なしの再登録で GitHub が消えた: %s / %s", got, *mem.value)
	}
	list := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLinkList, admin)))
	if !strings.Contains(list, "GitHub: octocat") {
		t.Fatalf("一覧に GitHub が無い: %s", list)
	}
}

func TestLink_管理者ロールが無いと全コマンドを拒否し何も書かない(t *testing.T) {
	mem := &memSSM{}
	h := newLinkHandler(mem, backlogStub{users: testUsers})
	for _, name := range []string{CommandNameLink, CommandNameUnlink, CommandNameLinkList} {
		got := contentOf(h.handleCommand(context.Background(), linkCmd(name, []string{"other"}, opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸"))))
		if !strings.Contains(got, "権限がありません") {
			t.Errorf("%s: 拒否されない: %s", name, got)
		}
	}
	if mem.value != nil {
		t.Errorf("拒否したのに書き込みが起きた: %s", *mem.value)
	}
}

func TestLink_名前は完全一致のみ(t *testing.T) {
	h := newLinkHandler(&memSSM{}, backlogStub{users: testUsers})
	admin := []string{"role-allowed"}
	// 部分一致で拾うと別人に紐づく
	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin, opt(OptionNameMember, "111"), opt(OptionNameBacklog, "山田"))))
	if !strings.Contains(got, "いません") {
		t.Errorf("部分一致で通った: %s", got)
	}
}

func TestLink_同じBacklogユーザーを別のDiscordに紐づけられない(t *testing.T) {
	mem := &memSSM{}
	h := newLinkHandler(mem, backlogStub{users: testUsers})
	admin := []string{"role-allowed"}
	h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin, opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸")))
	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin, opt(OptionNameMember, "222"), opt(OptionNameBacklog, "大橋和幸"))))
	if !strings.Contains(got, "既に <@111>") {
		t.Errorf("二重登録を拒否しない: %s", got)
	}
	// 同じ組み合わせの再登録(上書き)は通る
	got = contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin, opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸"))))
	if !strings.Contains(got, "✅") {
		t.Errorf("同一の再登録が通らない: %s", got)
	}
}

func TestLink_同じGitHubログインを別のDiscordに大文字小文字を無視して紐づけられない(t *testing.T) {
	mem := &memSSM{}
	h := newLinkHandler(mem, backlogStub{users: testUsers})
	admin := []string{"role-allowed"}
	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin,
		opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸"), opt(OptionNameGitHub, "Octocat"))))
	if !strings.Contains(got, "✅") {
		t.Fatalf("登録に失敗: %s", got)
	}
	got = contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin,
		opt(OptionNameMember, "222"), opt(OptionNameBacklog, "山田太郎"), opt(OptionNameGitHub, "@OCTOCAT"))))
	if strings.Contains(got, "✅") || !strings.Contains(got, "既に <@111>") {
		t.Errorf("GitHub の重複を拒否しない: %s", got)
	}
	if strings.Contains(*mem.value, "222") {
		t.Errorf("拒否したのに保存された: %s", *mem.value)
	}
	// 同じ人の再登録（表記違い）は通る
	got = contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, admin,
		opt(OptionNameMember, "111"), opt(OptionNameBacklog, "大橋和幸"), opt(OptionNameGitHub, "octocat"))))
	if !strings.Contains(got, "✅") {
		t.Errorf("同じ人の再登録が通らない: %s", got)
	}
}

func TestLinkService_GitHub未設定どうしは重複にしない(t *testing.T) {
	s := &LinkService{client: &memSSM{}, paramName: "/p/links"}
	ctx := context.Background()
	if err := s.Set(ctx, "111", Link{BacklogID: 1, Name: "大橋和幸"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, "222", Link{BacklogID: 2, Name: "山田太郎"}); err != nil {
		t.Errorf("GitHub 未設定どうしが衝突した: %v", err)
	}
	if err := s.Set(ctx, "111", Link{BacklogID: 1, Name: "大橋和幸", GitHub: "Octocat"}); err != nil {
		t.Fatal(err)
	}
	err := s.Set(ctx, "222", Link{BacklogID: 2, Name: "山田太郎", GitHub: " octocat "})
	if err == nil || !strings.Contains(err.Error(), "既に <@111>") {
		t.Errorf("大文字小文字違いの重複を拒否しない: %v", err)
	}
}

func TestLink_一覧はDiscordの文字数上限に収める(t *testing.T) {
	mem := &memSSM{}
	// 2000 文字を超える一覧になる人数と名前の長さ。
	links := map[string]Link{}
	for i := 0; i < 40; i++ {
		id := strings.Repeat("1", 18) + string(rune('a'+i%26)) + strings.Repeat("x", i/26)
		links[id] = Link{BacklogID: i + 1, Name: strings.Repeat("山", 25) + string(rune('A'+i%26)) + string(rune('a'+i/26))}
	}
	b, _ := json.Marshal(links)
	v := string(b)
	mem.value = &v
	h := newLinkHandler(mem, backlogStub{users: testUsers})
	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLinkList, []string{"role-allowed"})))
	if n := len([]rune(got)); n > 2000 {
		t.Errorf("一覧が %d 文字で Discord の上限を超える", n)
	}
	// 収まらない分は切り捨てず、次のページに回す（SOCAIAGENT-511）
	if !strings.Contains(got, "1/2ページ") {
		t.Errorf("続きのページがあると分からない: %s", got)
	}
}

func TestLink_Backlog障害の内部情報をDiscordへ返さない(t *testing.T) {
	h := newLinkHandler(&memSSM{}, backlogStub{err: errors.New("secret-internal-detail")})
	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, []string{"role-allowed"}, opt(OptionNameMember, "1"), opt(OptionNameBacklog, "x"))))
	if strings.Contains(got, "secret-internal-detail") {
		t.Errorf("内部エラーが漏れた: %s", got)
	}
}

func TestLink_壊れた対応表は上書きせず止める(t *testing.T) {
	broken := "{not json"
	mem := &memSSM{value: &broken}
	h := newLinkHandler(mem, backlogStub{users: testUsers})
	got := contentOf(h.handleCommand(context.Background(), linkCmd(CommandNameLink, []string{"role-allowed"}, opt(OptionNameMember, "1"), opt(OptionNameBacklog, "大橋和幸"))))
	if strings.Contains(got, "✅") || *mem.value != broken {
		t.Errorf("壊れた表を上書きした: resp=%s value=%s", got, *mem.value)
	}
}

func TestLinkService_上限を超える登録は拒否する(t *testing.T) {
	s := &LinkService{client: &memSSM{}, paramName: "/p/links"}
	var err error
	for i := 0; i < 200 && err == nil; i++ {
		err = s.Set(context.Background(), strings.Repeat("9", 18)+string(rune('a'+i%26))+strings.Repeat("x", i/26), Link{BacklogID: i + 1, Name: "山田太郎山田太郎"})
	}
	if err == nil || !strings.Contains(err.Error(), "上限") {
		t.Errorf("上限で止まらない: %v", err)
	}
}

func TestAutocomplete(t *testing.T) {
	h := newLinkHandler(&memSSM{}, backlogStub{users: testUsers})
	ac := func(roles []string, typed string) []Choice {
		i := linkCmd(CommandNameLink, roles, CommandOption{Name: OptionNameBacklog, Value: typed, Focused: true})
		i.Type = InteractionTypeAutocomplete
		r := h.handleAutocomplete(context.Background(), i)
		if r.Type != ResponseTypeAutocompleteResult {
			t.Fatalf("type=%d", r.Type)
		}
		return r.Data.Choices
	}
	if got := ac([]string{"role-allowed"}, "山田"); len(got) != 2 {
		t.Errorf("部分一致の候補が2件でない: %v", got)
	}
	if got := ac([]string{"role-allowed"}, ""); len(got) != 3 {
		t.Errorf("未入力は全員: %v", got)
	}
	if got := ac([]string{"other"}, "山田"); len(got) != 0 {
		t.Errorf("権限の無い人に候補が見えた: %v", got)
	}
}

func TestBacklogClient_エラーにAPIキーを含めない(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // 接続失敗を起こす
	c := &BacklogClient{baseURL: url, apiKey: "SECRETKEY123", projectKey: "P", http: &http.Client{}}
	_, err := c.Users(context.Background())
	if err == nil || strings.Contains(err.Error(), "SECRETKEY123") {
		t.Errorf("エラーにキーが含まれる、またはエラーが出ない: %v", err)
	}

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apiKey") != "SECRETKEY123" || !strings.HasSuffix(r.URL.Path, "/projects/P/users") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"name":"大橋和幸","userId":"x"}]`))
	}))
	defer srv.Close()
	c.baseURL = srv.URL
	users, err := c.Users(context.Background())
	if err != nil || len(users) != 1 || users[0].ID != 1 {
		t.Errorf("取得できない: %v %v", users, err)
	}
}
