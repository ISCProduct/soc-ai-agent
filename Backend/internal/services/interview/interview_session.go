package interview

import (
	"log"

	"Backend/internal/models"
	"Backend/internal/services/shared"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

// ErrInterviewBudgetExceeded は月次の面接上限に達したことを表す。
// 既定では上限に達しても止めない（監視のみ）。costs.InterviewBudgetService を参照。
var ErrInterviewBudgetExceeded = errors.New("interview monthly budget exceeded")

// maxClientUtteranceIDLen は client_utterance_id の上限。DBの VARCHAR(64) と揃える。
const maxClientUtteranceIDLen = 64

// 発話の上限（#1636）。STT の1発話は長くても数百字。30分の面接で往復は20前後。
// ponytail: 件数判定は保存済み発話を全件読む。上限が小さいので Count クエリは足さない。
const (
	maxUtteranceRunes       = 2000
	maxUtterancesPerSession = 200
)

var (
	ErrUtteranceTooLong  = errors.New("utterance too long")
	ErrTooManyUtterances = errors.New("too many utterances in session")
)

// interviewBudgetGuard は月次上限の判定面。未設定なら常に許可する。
type interviewBudgetGuard interface {
	AllowStart() error
}

// SetBudgetGuard は月次上限の判定を注入する（任意）。
func (s *InterviewService) SetBudgetGuard(g interviewBudgetGuard) {
	s.budgetGuard = g
}

func (s *InterviewService) CreateSession(userID uint, language string, interviewerGender string) (*InterviewSessionResponse, error) {
	// 面接1回ごとに STT / LLM / TTS を叩くため、セッション数がそのまま費用に効く。
	// 既定は監視のみで、止めるのは INTERVIEW_BUDGET_ENFORCE を明示したときだけ。
	if s.budgetGuard != nil {
		if err := s.budgetGuard.AllowStart(); err != nil {
			return nil, ErrInterviewBudgetExceeded
		}
	}
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	// 対応言語のホワイトリスト。以前は空文字しか弾いておらず、未知の値が
	// 採点プロンプトへ生で入った（normalizeLanguage のコメント参照、#1600）。
	language = normalizeLanguage(language)
	// 面接官は男性のみ。未指定・不正値は male（声も male 側になる）。
	if interviewerGender != "male" && interviewerGender != "female" {
		interviewerGender = "male"
	}
	session := &models.InterviewSession{
		UserID:            userID,
		Status:            "ready",
		Language:          language,
		InterviewerGender: interviewerGender,
		TemplateVersion:   shared.GetEnv("INTERVIEW_TEMPLATE_VERSION", "v1"),
	}
	if err := s.sessionRepo.Create(session); err != nil {
		return nil, err
	}
	return toSessionResponse(session), nil
}

func (s *InterviewService) StartSession(userID uint, sessionID uint) (*InterviewSessionResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, shared.ErrForbidden
	}
	if session.Status == "finished" {
		return nil, shared.ErrSessionFinished
	}
	if session.StartedAt == nil {
		now := time.Now()
		session.StartedAt = &now
	}
	session.Status = "in_progress"
	if err := s.sessionRepo.Update(session); err != nil {
		return nil, err
	}
	return toSessionResponse(session), nil
}

func (s *InterviewService) FinishSession(userID uint, sessionID uint) (*InterviewSessionResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, shared.ErrForbidden
	}
	if session.Status == "finished" {
		// 既に終了済み: レポート再キューなどの副作用を起こさず、現在の状態をそのまま返す(#1019, 冪等)。
		return toSessionResponse(session), nil
	}
	if session.EndedAt == nil {
		now := time.Now()
		session.EndedAt = &now
	}
	session.Status = "finished"
	if s.realtimeUsageService != nil {
		if durationSec, cost, err := s.realtimeUsageService.CloseSession(sessionID, *session.EndedAt, nil); err == nil && durationSec >= 0 {
			session.EstimatedCostUSD = cost
		} else {
			session.EstimatedCostUSD = s.estimateCost(session.StartedAt, session.EndedAt)
		}
	} else {
		session.EstimatedCostUSD = s.estimateCost(session.StartedAt, session.EndedAt)
	}
	if err := s.sessionRepo.Update(session); err != nil {
		return nil, err
	}
	s.enqueueReportGeneration(sessionID)
	return toSessionResponse(session), nil
}

// EnsureSessionOwnership はセッションが userID の所有物（または管理者操作）であることを検証する。
// IDOR対策: URLパスの sessionID を扱うハンドラは、実処理の前に必ずこれを呼び出すこと。
func (s *InterviewService) EnsureSessionOwnership(userID uint, sessionID uint) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	if !s.isAllowed(userID, session.UserID) {
		return shared.ErrForbidden
	}
	return nil
}

// SaveUtterance はクライアントが申告した発話を保存する。clientUtteranceID は発話ごとにクライアントが
// 発行する一意ID(#1476)。
//
// 保存するのは role=user だけ（#1969）。面接官の発話は /start-turn・/turn でサーバーが保存する
// （saveTurnUtterances）。クライアントの申告を保存すると、面接官が言っていない質問を書き起こしに
// 混ぜられ、レポートと evidence の照合元が汚れる。
//
// role=ai は保存せずに成功を返す。デプロイ前から開いている画面はターンのたびに user/ai を送るので、
// エラーにすると「記録できていない」と表示され続ける。同じ理由で、直前にサーバーが保存した
// ユーザー発話と同じ文面の role=user も保存しない（二重保存を防ぐ）。
//
// 保存は結果不明のまま失敗しうる（応答のロスト、クライアント側タイムアウト）ため、
// クライアントは同じ ID を付けたまま再送する。重複判定はサーバー側でしかできないので
// (session_id, client_utterance_id) の一意制約に委ね、当たった再送は no-op にする。
// 空文字は NULL として保存し、ID を送らない経路は従来どおり追記される。
func (s *InterviewService) SaveUtterance(userID uint, sessionID uint, role string, text string, clientUtteranceID string) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	if !s.isAllowed(userID, session.UserID) {
		return shared.ErrForbidden
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "ai" {
		log.Printf("[Interview] client role=ai utterance ignored session=%d", sessionID)
		return nil
	}
	if role != "user" {
		return errors.New("invalid role")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty text")
	}
	// 発話はスコアとレポートの入力で、面接官の履歴にもなる（#1636）。
	// 長さと件数に上限が無いと、1リクエストで文脈を埋め尽くせる。
	if utf8.RuneCountInString(text) > maxUtteranceRunes {
		return ErrUtteranceTooLong
	}
	existing, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return err
	}
	if duplicatesServerSavedUserUtterance(existing, text) {
		return nil
	}
	if len(existing) >= maxUtterancesPerSession {
		return ErrTooManyUtterances
	}
	utter := &models.InterviewUtterance{
		SessionID: sessionID,
		Role:      role,
		Text:      text,
	}
	if id := strings.TrimSpace(clientUtteranceID); id != "" {
		if len(id) > maxClientUtteranceIDLen {
			return errors.New("client_utterance_id too long")
		}
		utter.ClientUtteranceID = &id
	}
	return s.utterRepo.Create(utter)
}

// turnUtterance はターンでサーバーが得た発話。
type turnUtterance struct {
	role string
	text string
}

// serverUtteranceIDPrefix はサーバーが保存した発話の client_utterance_id の接頭辞（#1969）。
// 再試行と同じターンの再実行を冪等にするために、ターンと順番から ID を決める。
// クライアントの ID（UUID）とは重ならない。
const serverUtteranceIDPrefix = "srv-"

// turnUtteranceSaveAttempts は1ターンの発話を保存する最大回数。
const turnUtteranceSaveAttempts = 3

// turnUtteranceRetryWait は再試行までの待ち時間の単位。2回目はこの1倍、3回目は2倍待つ。
var turnUtteranceRetryWait = 200 * time.Millisecond

// saveTurnUtterances は /start-turn・/turn でサーバーが得た発話を、渡した順に保存する（#1969）。
//
// /turn ではユーザーの発話もサーバーの文字起こしなので、面接官の発話と一緒にここで保存する。
// クライアントが応答を受け取ってからユーザーの発話を保存すると、サーバーが先に保存した
// 面接官の発話より後ろに並び、書き起こしの順番が入れ替わる。
//
// 1ターンの発話は1つのトランザクションで保存し、一時的な DB の失敗はまとめて再試行する。
// user だけ残って ai が欠けると、次のターンの履歴から直前の質問が消えるため。
//
// 発話の ID は turnKey（ターンを始めたときの保存済み件数から決める）と順番で決める。同じターンが
// 応答の前にもう一度走っても同じ ID になり、一意制約で二重に保存されない。結果が不明な失敗の後の
// 再試行も同じ扱いになる（以前のクライアントの再試行と同じ）。
//
// 長さと件数の上限は SaveUtterance と揃える。長すぎる発話は切り詰め、上限に達したら
// ErrTooManyUtterances を返す。空の発話は保存しない。
func (s *InterviewService) saveTurnUtterances(sessionID uint, turnKey string, utterances ...turnUtterance) error {
	if s.utterRepo == nil {
		return errors.New("utterance repository is not configured")
	}
	existing, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return err
	}
	count := len(existing)
	rows := make([]models.InterviewUtterance, 0, len(utterances))
	for i, u := range utterances {
		text := strings.TrimSpace(u.text)
		if text == "" {
			continue
		}
		if count >= maxUtterancesPerSession {
			return ErrTooManyUtterances
		}
		if runes := []rune(text); len(runes) > maxUtteranceRunes {
			text = string(runes[:maxUtteranceRunes])
		}
		id := fmt.Sprintf("%s%s-%d", serverUtteranceIDPrefix, turnKey, i)
		rows = append(rows, models.InterviewUtterance{
			SessionID: sessionID, Role: u.role, Text: text, ClientUtteranceID: &id,
		})
		count++
	}
	if len(rows) == 0 {
		return nil
	}
	return s.createUtterancesWithRetry(rows)
}

// utteranceBatchCreator は1ターンの発話を1つのトランザクションで保存できるリポジトリ
// （repositories.InterviewUtteranceRepository が満たす）。
type utteranceBatchCreator interface {
	CreateAll(utterances []*models.InterviewUtterance) error
}

// createUtterancesWithRetry は1ターンの発話を最大 turnUtteranceSaveAttempts 回保存する。
// 途中で接続が切れても、保存を始めた発話は最後まで試す。
func (s *InterviewService) createUtterancesWithRetry(rows []models.InterviewUtterance) error {
	var err error
	for attempt := 1; attempt <= turnUtteranceSaveAttempts; attempt++ {
		if err = s.createUtterances(rows); err == nil {
			return nil
		}
		if attempt < turnUtteranceSaveAttempts {
			log.Printf("[Interview] save turn utterances retry session=%d attempt=%d: %v", rows[0].SessionID, attempt, err)
			time.Sleep(time.Duration(attempt) * turnUtteranceRetryWait)
		}
	}
	return err
}

// createUtterances は rows を保存する。リポジトリがトランザクションでの保存に対応していれば
// それを使い、全件か0件のどちらかにする。
func (s *InterviewService) createUtterances(rows []models.InterviewUtterance) error {
	ptrs := make([]*models.InterviewUtterance, len(rows))
	for i := range rows {
		row := rows[i]
		ptrs[i] = &row
	}
	if batch, ok := s.utterRepo.(utteranceBatchCreator); ok {
		return batch.CreateAll(ptrs)
	}
	for _, row := range ptrs {
		if err := s.utterRepo.Create(row); err != nil {
			return err
		}
	}
	return nil
}

// turnUtteranceKey は、ターンの発話の ID に使うキーを返す。kind は "start"（冒頭の質問）か
// "turn"（回答と次の質問）、savedCount はターンを始めたときの保存済みの発話の件数。
// 同じターンの再実行は同じキーになる。保存済みの件数を読めなかったときは、二重保存の防止より
// 保存を優先して毎回別のキーにする。
func turnUtteranceKey(kind string, savedCount int, loaded bool) string {
	if !loaded {
		return kind + "-" + uuid.NewString()
	}
	return fmt.Sprintf("%s-%d", kind, savedCount)
}

// duplicatesServerSavedUserUtterance は、最後のユーザー発話がサーバーの保存したもので、
// 文面が text と同じかを返す。デプロイ前から開いている画面が、サーバーの保存済みの回答を
// もう一度送ってきた場合に当たる。
func duplicatesServerSavedUserUtterance(existing []models.InterviewUtterance, text string) bool {
	for i := len(existing) - 1; i >= 0; i-- {
		u := existing[i]
		if u.Role != "user" {
			continue
		}
		return u.ClientUtteranceID != nil &&
			strings.HasPrefix(*u.ClientUtteranceID, serverUtteranceIDPrefix) &&
			strings.TrimSpace(u.Text) == text
	}
	return false
}

// ListAllSessionsAdmin lists all interview sessions without performing a user-level admin check.
// The caller (admin middleware) is responsible for ensuring only admins can invoke this.
func (s *InterviewService) ListAllSessionsAdmin(limit int, offset int, schoolID *uint, companyID *uint) ([]InterviewSessionResponse, int64, error) {
	total, err := s.sessionRepo.CountAll(schoolID, companyID)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := s.sessionRepo.ListAll(limit, offset, schoolID, companyID)
	if err != nil {
		return nil, 0, err
	}
	return toSessionResponses(sessions), total, nil
}

// ListSessionsForOwner は企業オーナー向け面接一覧。company_id 必須、所有権がなければ 403。
func (s *InterviewService) ListSessionsForOwner(userID, companyID uint, limit int, offset int) ([]InterviewSessionResponse, int64, error) {
	if companyID == 0 || s.ownsCompany == nil {
		return nil, 0, shared.ErrForbidden
	}
	ok, err := s.ownsCompany(userID, companyID)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, shared.ErrForbidden
	}
	cid := companyID
	return s.ListAllSessionsAdmin(limit, offset, nil, &cid)
}

func (s *InterviewService) ListSessions(userID uint, all bool, limit int, offset int) ([]InterviewSessionResponse, int64, error) {
	if all {
		user, err := s.userRepo.GetUserByID(userID)
		if err != nil || user == nil || !user.IsAdmin {
			return nil, 0, shared.ErrForbidden
		}
		total, err := s.sessionRepo.CountAll(nil, nil)
		if err != nil {
			return nil, 0, err
		}
		sessions, err := s.sessionRepo.ListAll(limit, offset, nil, nil)
		if err != nil {
			return nil, 0, err
		}
		return toSessionResponses(sessions), total, nil
	}
	total, err := s.sessionRepo.CountByUser(userID)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := s.sessionRepo.ListByUser(userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return toSessionResponses(sessions), total, nil
}

// GetSessionDetail は面接詳細を返す。教員用レポートを含めるかは呼び出し元の実ロールで
// サーバー側が決める（クライアント送信の値は使わない。#1814）。
func (s *InterviewService) GetSessionDetail(userID uint, sessionID uint) (*InterviewDetailResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, shared.ErrForbidden
	}
	utterances, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	var report *models.InterviewReport
	report, err = s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		report = nil
	}
	report = s.redactTeacherReport(userID, report)
	return &InterviewDetailResponse{
		Session:    *toSessionResponse(session),
		Utterances: utterances,
		Report:     report,
	}, nil
}
