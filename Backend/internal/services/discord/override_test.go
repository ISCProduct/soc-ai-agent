package discord

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestParseOverride(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"on", "on", OverrideOn, false},
		{"off", "off", OverrideOff, false},
		{"auto", "auto", OverrideAuto, false},
		{"大文字も受理", "ON", OverrideOn, false},
		{"前後空白を無視", "  off  ", OverrideOff, false},
		{"未指定は auto", "", OverrideAuto, false},
		// 「本番を止める/起動する」判断なので、曖昧な入力は通さない
		{"yes は受理しない", "yes", "", true},
		{"true は受理しない", "true", "", true},
		{"1 は受理しない", "1", "", true},
		{"stop は受理しない", "stop", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOverride(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseOverride(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseOverride(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestFindOptionString(t *testing.T) {
	options := []CommandOption{
		{Name: "other", Value: "x"},
		{Name: OptionNameState, Value: "on"},
	}
	if got := FindOptionString(options, OptionNameState); got != "on" {
		t.Errorf("FindOptionString() = %q, want %q", got, "on")
	}
	if got := FindOptionString(options, "missing"); got != "" {
		t.Errorf("FindOptionString() for missing = %q, want empty", got)
	}
	// 文字列以外が来ても panic せず空を返す（他コマンドの数値引数などを想定）
	if got := FindOptionString([]CommandOption{{Name: OptionNameState, Value: 42}}, OptionNameState); got != "" {
		t.Errorf("FindOptionString() for non-string = %q, want empty", got)
	}
	if got := FindOptionString(nil, OptionNameState); got != "" {
		t.Errorf("FindOptionString(nil) = %q, want empty", got)
	}
}

func TestWorkflowDispatcher_Enabled(t *testing.T) {
	tests := []struct {
		name  string
		token string
		repo  string
		want  bool
	}{
		{"両方あれば有効", "t", "o/r", true},
		{"トークンなし", "", "o/r", false},
		{"リポジトリなし", "t", "", false},
		{"どちらもなし", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &WorkflowDispatcher{token: tt.token, repo: tt.repo}
			if got := d.Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
	// nil でも panic しない（未設定時に呼ばれる経路がある）
	var nilDispatcher *WorkflowDispatcher
	if nilDispatcher.Enabled() {
		t.Error("nil dispatcher should not be enabled")
	}
}

func TestWorkflowDispatcher_Dispatch(t *testing.T) {
	t.Run("未設定なら起動せずエラーにもしない", func(t *testing.T) {
		d := &WorkflowDispatcher{}
		dispatched, err := d.Dispatch(context.Background())
		if err != nil || dispatched {
			t.Errorf("Dispatch() = (%v, %v), want (false, nil)", dispatched, err)
		}
	})

	t.Run("204 なら成功", func(t *testing.T) {
		var gotAuth, gotPath, gotBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			gotBody = string(buf)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		d := newTestDispatcher(srv.URL, "tok", "own/repo", "wf.yml", "main")
		dispatched, err := d.Dispatch(context.Background())
		if err != nil || !dispatched {
			t.Fatalf("Dispatch() = (%v, %v), want (true, nil)", dispatched, err)
		}
		if gotAuth != "Bearer tok" {
			t.Errorf("Authorization = %q", gotAuth)
		}
		if gotPath != "/repos/own/repo/actions/workflows/wf.yml/dispatches" {
			t.Errorf("path = %q", gotPath)
		}
		if gotBody != `{"ref":"main"}` {
			t.Errorf("body = %q", gotBody)
		}
	})

	t.Run("204以外は失敗として返す", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// トークンの権限不足はこの経路で起きる。成功扱いにすると
			// 「反映した」と返したまま本番が変わらない
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		d := newTestDispatcher(srv.URL, "tok", "own/repo", "wf.yml", "main")
		dispatched, err := d.Dispatch(context.Background())
		if err == nil || dispatched {
			t.Errorf("Dispatch() = (%v, %v), want (false, error)", dispatched, err)
		}
	})
}

// newTestDispatcher はGitHub APIのURLを差し替えたDispatcherを作る（テスト専用）。
func newTestDispatcher(baseURL, token, repo, workflow, ref string) *WorkflowDispatcher {
	return &WorkflowDispatcher{
		token:    token,
		repo:     repo,
		workflow: workflow,
		ref:      ref,
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// fakeSSM はSSMの応答を再現する。
type fakeSSM struct {
	value    string
	getErr   error
	putValue string
	putCalls int
}

func (f *fakeSSM) GetParameter(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &ssm.GetParameterOutput{Parameter: &ssmtypes.Parameter{Value: aws.String(f.value)}}, nil
}

func (f *fakeSSM) PutParameter(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	f.putCalls++
	f.putValue = aws.ToString(in.Value)
	return &ssm.PutParameterOutput{}, nil
}

func newTestUptimeService(f *fakeSSM) *UptimeService {
	return &UptimeService{client: f, parameterName: "/p/dates", overrideParameterName: "/p/override"}
}

func TestUptimeService_GetOverride(t *testing.T) {
	tests := []struct {
		name  string
		value string
		err   error
		want  string
	}{
		{"on", "on", nil, OverrideOn},
		{"off", "off", nil, OverrideOff},
		{"auto", "auto", nil, OverrideAuto},
		{"大文字は正規化する", "ON", nil, OverrideOn},
		{"前後空白は無視する", " off ", nil, OverrideOff},
		// SSMは手でも書ける。未知の値を on/off と解釈すると、
		// 打ち間違い一つで本番が起動しっぱなし/落ちたままになる。
		{"未知の値は auto に倒す", "yes", nil, OverrideAuto},
		{"空文字は auto", "", nil, OverrideAuto},
		{"パラメータ未作成は auto", "", &ssmtypes.ParameterNotFound{}, OverrideAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestUptimeService(&fakeSSM{value: tt.value, getErr: tt.err})
			got, err := s.GetOverride(context.Background())
			if err != nil {
				t.Fatalf("GetOverride() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("GetOverride() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("ParameterNotFound以外のエラーは握り潰さない", func(t *testing.T) {
		s := newTestUptimeService(&fakeSSM{getErr: errors.New("AccessDeniedException")})
		if _, err := s.GetOverride(context.Background()); err == nil {
			t.Error("GetOverride() error = nil, want error")
		}
	})
}

func TestUptimeService_SetOverride(t *testing.T) {
	t.Run("正規化した値を書き込む", func(t *testing.T) {
		f := &fakeSSM{}
		if err := newTestUptimeService(f).SetOverride(context.Background(), " ON "); err != nil {
			t.Fatalf("SetOverride() error = %v", err)
		}
		if f.putValue != OverrideOn {
			t.Errorf("書き込まれた値 = %q, want %q", f.putValue, OverrideOn)
		}
	})

	// 検証を外すと、SSMに on/off/auto 以外が入りうる。
	// ワークフローはそれを auto として扱うので、Discordの表示と実挙動がずれる。
	t.Run("不正な値は書き込まない", func(t *testing.T) {
		f := &fakeSSM{}
		if err := newTestUptimeService(f).SetOverride(context.Background(), "stop"); err == nil {
			t.Error("SetOverride() error = nil, want error")
		}
		if f.putCalls != 0 {
			t.Errorf("PutParameter が %d 回呼ばれた。不正な値では書き込まないこと", f.putCalls)
		}
	})
}
