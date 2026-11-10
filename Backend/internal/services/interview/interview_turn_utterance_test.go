package interview

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// #1969: 面接官の発話はサーバーが保存し、クライアントの申告（role=ai）は受け付けない。

func newTurnTestServer(t *testing.T, transcript, reply string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			_, _ = w.Write([]byte(`{"text":"` + transcript + `"}`))
		case "/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + reply + `"}}]}`))
		case "/audio/speech":
			_, _ = w.Write([]byte("audio"))
		default:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func utteranceLines(utterances []models.InterviewUtterance) []string {
	lines := make([]string, 0, len(utterances))
	for _, u := range utterances {
		lines = append(lines, u.Role+":"+u.Text)
	}
	return lines
}

// クライアントの role=ai は保存しない。デプロイ前から開いている画面に警告を出し続けないよう、
// エラーにはせず成功を返す。
func TestSaveUtterance_IgnoresClientAIUtterance(t *testing.T) {
	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress"})
	utters := &utterRepoStub{}
	svc := NewInterviewService(repo, utters, nil, nil, nil, nil, nil)

	for _, role := range []string{"ai", " AI "} {
		if err := svc.SaveUtterance(10, 1, role, "次の質問です", ""); err != nil {
			t.Fatalf("role=%q: err = %v, want nil", role, err)
		}
	}
	if err := svc.SaveUtterance(10, 1, "user", "自己紹介をします", ""); err != nil {
		t.Fatalf("role=user は保存できること: %v", err)
	}
	if got := utteranceLines(utters.utterances); strings.Join(got, "|") != "user:自己紹介をします" {
		t.Fatalf("保存された発話 = %v", got)
	}
}

// デプロイ前から開いている画面は、サーバーが保存済みの回答をもう一度送ってくる。二重に保存しない。
func TestSaveUtterance_SkipsResentServerSavedUserUtterance(t *testing.T) {
	serverID := serverUtteranceIDPrefix + "x"
	clientID := "client-id"
	tests := []struct {
		name     string
		existing []models.InterviewUtterance
		text     string
		wantSave bool
	}{
		{
			name: "サーバーが保存した最後の回答と同じ文面は保存しない",
			existing: []models.InterviewUtterance{
				{Role: "user", Text: "回答です", ClientUtteranceID: &serverID},
				{Role: "ai", Text: "次の質問です", ClientUtteranceID: &serverID},
			},
			text: "回答です",
		},
		{
			name:     "文面が違えば保存する",
			existing: []models.InterviewUtterance{{Role: "user", Text: "回答です", ClientUtteranceID: &serverID}},
			text:     "別の回答です", wantSave: true,
		},
		{
			name:     "最後の回答がクライアントの保存なら保存する",
			existing: []models.InterviewUtterance{{Role: "user", Text: "回答です", ClientUtteranceID: &clientID}},
			text:     "回答です", wantSave: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utters := &utterRepoStub{utterances: append([]models.InterviewUtterance(nil), tt.existing...)}
			svc := NewInterviewService(newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress"}), utters, nil, nil, nil, nil, nil)
			if err := svc.SaveUtterance(10, 1, "user", tt.text, "new-client-id"); err != nil {
				t.Fatalf("err = %v", err)
			}
			if saved := len(utters.utterances) > len(tt.existing); saved != tt.wantSave {
				t.Fatalf("保存した = %v, want %v", saved, tt.wantSave)
			}
		})
	}
}

// flakyUtterRepo は最初の failures 回だけ保存に失敗する。
type flakyUtterRepo struct {
	utterRepoStub
	failures int
	calls    int
}

func (r *flakyUtterRepo) Create(u *models.InterviewUtterance) error {
	r.calls++
	if r.calls <= r.failures {
		return errors.New("deadlock")
	}
	return r.utterRepoStub.Create(u)
}

func TestSaveTurnUtterances_一時的な失敗は再試行する(t *testing.T) {
	prev := turnUtteranceRetryWait
	turnUtteranceRetryWait = 0
	t.Cleanup(func() { turnUtteranceRetryWait = prev })

	t.Run("2回失敗しても3回目で保存する", func(t *testing.T) {
		repo := &flakyUtterRepo{failures: 2}
		svc := NewInterviewService(nil, repo, nil, nil, nil, nil, nil)
		if err := svc.saveTurnUtterances(1, turnUtterance{role: "user", text: "回答"}); err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(repo.utterances) != 1 || repo.calls != 3 {
			t.Fatalf("保存=%d 呼び出し=%d", len(repo.utterances), repo.calls)
		}
	})
	t.Run("3回とも失敗したらエラーを返す", func(t *testing.T) {
		repo := &flakyUtterRepo{failures: 3}
		svc := NewInterviewService(nil, repo, nil, nil, nil, nil, nil)
		if err := svc.saveTurnUtterances(1, turnUtterance{role: "user", text: "回答"}); err == nil {
			t.Fatal("エラーにならない")
		}
		if repo.calls != turnUtteranceSaveAttempts {
			t.Fatalf("呼び出し=%d", repo.calls)
		}
	})
}

func TestSaveTurnUtterances(t *testing.T) {
	t.Run("渡した順に保存し、空は飛ばし、長すぎる発話は切り詰める", func(t *testing.T) {
		utters := &utterRepoStub{}
		svc := NewInterviewService(nil, utters, nil, nil, nil, nil, nil)
		long := strings.Repeat("あ", maxUtteranceRunes+10)
		err := svc.saveTurnUtterances(1,
			turnUtterance{role: "user", text: " 回答です "},
			turnUtterance{role: "ai", text: ""},
			turnUtterance{role: "ai", text: long},
		)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(utters.utterances) != 2 {
			t.Fatalf("件数 = %d, want 2", len(utters.utterances))
		}
		// 再試行を冪等にするため、サーバーが振った ID を付けて保存する
		if u := utters.utterances[0]; u.Role != "user" || u.Text != "回答です" || u.SessionID != 1 ||
			u.ClientUtteranceID == nil || !strings.HasPrefix(*u.ClientUtteranceID, serverUtteranceIDPrefix) {
			t.Fatalf("1件目 = %+v", u)
		}
		if *utters.utterances[0].ClientUtteranceID == *utters.utterances[1].ClientUtteranceID {
			t.Fatal("発話ごとに別の ID を振ること")
		}
		if u := utters.utterances[1]; u.Role != "ai" || len([]rune(u.Text)) != maxUtteranceRunes {
			t.Fatalf("2件目 role=%s 長さ=%d", u.Role, len([]rune(u.Text)))
		}
	})

	t.Run("件数の上限に達したら保存しない", func(t *testing.T) {
		utters := &utterRepoStub{utterances: make([]models.InterviewUtterance, maxUtterancesPerSession-1)}
		svc := NewInterviewService(nil, utters, nil, nil, nil, nil, nil)
		err := svc.saveTurnUtterances(1, turnUtterance{role: "user", text: "回答"}, turnUtterance{role: "ai", text: "質問"})
		if !errors.Is(err, ErrTooManyUtterances) {
			t.Fatalf("err = %v, want ErrTooManyUtterances", err)
		}
		if len(utters.utterances) != maxUtterancesPerSession {
			t.Fatalf("件数 = %d, want %d", len(utters.utterances), maxUtterancesPerSession)
		}
	})
}

// /turn はユーザー → 面接官の順に書き起こしへ保存する。クライアントが後からユーザーの
// 発話を保存すると、面接官の発話の後ろに並んで順番が入れ替わる。
func TestTurn_SavesUserThenAIUtterance(t *testing.T) {
	server := newTurnTestServer(t, "チームで開発しました", "具体的には何をしましたか")
	utters := &utterRepoStub{utterances: []models.InterviewUtterance{{SessionID: 1, Role: "ai", Text: "自己紹介をお願いします"}}}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		utters, nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil,
	)

	result, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("Turn error: %v", err)
	}
	want := []string{"ai:自己紹介をお願いします", "user:チームで開発しました", "ai:具体的には何をしましたか"}
	if got := utteranceLines(utters.utterances); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("書き起こし = %v, want %v", got, want)
	}
	if result.UtteranceSaveFailed {
		t.Fatal("保存できたのに UtteranceSaveFailed が立っている")
	}
}

// 保存に失敗しても面接は止めず、画面に知らせるために UtteranceSaveFailed を返す。
func TestTurn_ReportsUtteranceSaveFailure(t *testing.T) {
	server := newTurnTestServer(t, "回答です", "次の質問です")
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		&utterRepoStub{createErr: errors.New("db down")}, nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil,
	)

	result, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("保存の失敗でターンを止めないこと: %v", err)
	}
	if !result.UtteranceSaveFailed || result.AIText != "次の質問です" {
		t.Fatalf("result = %+v", result)
	}
}

func TestStartTurn_SavesOpeningQuestion(t *testing.T) {
	server := newTurnTestServer(t, "", "自己紹介をお願いします")
	utters := &utterRepoStub{}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		utters, nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil,
	)

	result, err := svc.StartTurn(context.Background(), 10, 1, "", "", "", "", "", 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("StartTurn error: %v", err)
	}
	if got := utteranceLines(utters.utterances); strings.Join(got, "|") != "ai:自己紹介をお願いします" || result.UtteranceSaveFailed {
		t.Fatalf("書き起こし = %v failed=%v", got, result.UtteranceSaveFailed)
	}
}

// クライアントが切断・タイムアウトしたターンは保存しない。届かなかった質問を書き起こしと
// 次の履歴に残すと、会話とレポートが利用者の体験とずれる。
func TestTurn_DoesNotSaveWhenClientIsGone(t *testing.T) {
	server := newTurnTestServer(t, "回答です", "次の質問です")
	utters := &utterRepoStub{}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		utters, nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := svc.Turn(ctx, 10, 1, []byte("audio"), "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, err := svc.StartTurn(ctx, 10, 1, "", "", "", "", "", 0, 1, 1, 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("StartTurn err = %v, want context.Canceled", err)
	}
	if len(utters.utterances) != 0 {
		t.Fatalf("保存された発話 = %v", utteranceLines(utters.utterances))
	}
}
