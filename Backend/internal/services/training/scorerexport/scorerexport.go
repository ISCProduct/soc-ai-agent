// Package scorerexport は、軽量採点モデルの学習データを書き出すジョブ（#2168。設計は #1834 の 2・4）。
//
// 学習に使ってよい学生（repositories.TrainingScorerExportRepository の trainingEligibleUsers）の、
// 完了した面接ごとに「マスキングした質問と回答の並び」と「LLM の採点（ラベル）」を JSONL の1行にして、
// 非公開の S3 プレフィックスに置く。書き出しのたびに監査ログ（training_scorer_export_runs）を残す。
//
// 既存の training.Service（選考結果を教師ラベルにする書き出し）とは別の目的なので、別のジョブにしている。
// あちらは「AI の採点を含めない」方針で、LLM の採点をラベルにする例外は、この書き出しに限って
// 認められている（#1834 の大橋さんの回答 5.）。
//
// # 出力の形
//
//	<prefix>/runs/<実行ID>/subjects/<subject>.jsonl  学生1人ぶん。1行が面接1回
//	<prefix>/runs/<実行ID>/manifest.json             最後に書く。これがある回だけが書き出しを終えた回
//
// subject は利用者 ID の HMAC。学生ごとにファイルを分けるのは、同意の撤回・退会のときに
// その学生の書き出し済みデータを、ほかの学生のデータを書き直さずに消せるようにするため（sweep.go）。
package scorerexport

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/interview"
	"Backend/internal/services/shared"
	"Backend/internal/services/training/masking"
)

// 設定の環境変数。
const (
	// EnvHMACKey は利用者 ID と面接 ID を HMAC にする秘密鍵。
	EnvHMACKey = "TRAINING_EXPORT_HMAC_KEY"
	// EnvDestination は出力先の S3 の URI（s3://bucket/prefix）。非公開のプレフィックスを指定する。
	EnvDestination = "TRAINING_EXPORT_S3_URI"
)

// MinHMACKeyBytes は秘密鍵の最小の長さ。HMAC-SHA256 の出力と同じ 32 バイト未満の鍵は、
// 総当たりで鍵を探される余地が大きくなる（鍵が分かると、利用者 ID から subject を作って突き合わせられる）。
const MinHMACKeyBytes = 32

// FormatVersion は JSONL の1行の形の版。形を変えたら上げる（学習スクリプトが読み分けるため）。
const FormatVersion = 1

// LabelSource はラベルの出どころ。#1834 の 5. の条件「LLM ラベルで学習と明記する」のため、各行とマニフェストに書く。
const LabelSource = "llm"

// labelNote はマニフェストに書く、ラベルの扱いの注意（#1834 の大橋さんの回答 5.）。
const labelNote = "labels は LLM の採点（interview_reports.scores_json）で、教員の採点ではない。" +
	"LLM ラベルで学習したことをレポートに明記し、精度の評価は教員が採点したセットで行う。他の学習用途には使わない。"

// 設定の誤り。CLI は errors.Is で見分けて表示する。
var (
	ErrHMACKeyMissing     = errors.New("scorerexport: " + EnvHMACKey + " が設定されていません")
	ErrHMACKeyTooShort    = fmt.Errorf("scorerexport: %s は %d バイト以上にしてください", EnvHMACKey, MinHMACKeyBytes)
	ErrDestinationMissing = errors.New("scorerexport: " + EnvDestination + " が設定されていません")
	ErrDestinationInvalid = errors.New("scorerexport: " + EnvDestination + " は s3://bucket/prefix の形で、プレフィックスまで指定してください")
)

// Config はジョブの設定。マスキングを外す設定は持たない（学習用の出力ではマスキングを必須にする。#2168 追記）。
type Config struct {
	HMACKey []byte
	Bucket  string
	// Prefix は出力先のプレフィックス。前後の「/」は除いて持つ。
	Prefix string
}

// ConfigFromEnv は環境変数から設定を読む。鍵か出力先が無ければエラーにする（ジョブを動かさない）。
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	// 前後の空白は落とす。空白だけの値を鍵として受け付けないため。
	cfg := Config{HMACKey: []byte(strings.TrimSpace(getenv(EnvHMACKey)))}
	uri := strings.TrimSpace(getenv(EnvDestination))
	if uri != "" {
		bucket, prefix, ok := shared.ParseS3URI(uri)
		if !ok {
			return Config{}, ErrDestinationInvalid
		}
		cfg.Bucket, cfg.Prefix = bucket, strings.Trim(prefix, "/")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate は設定を確かめる。
// プレフィックスを必須にするのは、バケットの直下に置くと、IAM で書き出し用の範囲だけを絞れないため。
func (c Config) Validate() error {
	switch {
	case len(c.HMACKey) == 0:
		return ErrHMACKeyMissing
	case len(c.HMACKey) < MinHMACKeyBytes:
		return ErrHMACKeyTooShort
	case c.Bucket == "" && c.Prefix == "":
		return ErrDestinationMissing
	case c.Bucket == "" || c.Prefix == "":
		return ErrDestinationInvalid
	}
	return nil
}

// Store はジョブが使う DB の操作。実装は repositories.TrainingScorerExportRepository。
type Store interface {
	IsTrainingEligible(ctx context.Context, userID uint) (bool, error)
	ListTrainingSubjects(ctx context.Context) ([]repositories.TrainingSubject, error)
	ListTrainingSessions(ctx context.Context, userID uint) ([]repositories.TrainingSession, error)
	ListTrainingUtterances(ctx context.Context, sessionID uint) ([]repositories.TrainingUtterance, error)
	ListTrainingMaskingTerms(ctx context.Context) (repositories.TrainingMaskingTerms, error)
	// SweepStore は、書き出しの前に行う「書き出し済みのデータの削除」（Sweeper.Sweep）で使う。
	SweepStore
	StartTrainingExportRun(ctx context.Context, destination string, startedAt time.Time) (uint, error)
	FinishTrainingExportRun(ctx context.Context, id uint, status string, students, sessions int, errMsg string, finishedAt time.Time) error
}

var _ Store = (*repositories.TrainingScorerExportRepository)(nil)

// ObjectWriter は出力先への書き込み。本番は S3（NewS3Writer）、テストでは差し替える。
type ObjectWriter interface {
	PutObject(ctx context.Context, key string, body []byte, contentType string) error
	// DeleteObject は書いたオブジェクトを消す。書いた後に同意の取り消しが分かったときに使う（#2325 のレビュー）。
	DeleteObject(ctx context.Context, key string) error
}

// Turn は AI の質問1つと、それに対する学生の回答。どちらもマスキング済み。
type Turn struct {
	Q string `json:"q"`
	A string `json:"a"`
}

// Sample は JSONL の1行（面接1回ぶん）。
type Sample struct {
	FormatVersion int `json:"format_version"`
	// SampleID は面接 ID の HMAC。DB の面接 ID を出力に書かないため。
	SampleID string `json:"sample_id"`
	// Subject は利用者 ID の HMAC。学習と評価を利用者単位で分けるのに使う（#1834 の 3.）。
	Subject         string         `json:"subject"`
	TemplateVersion string         `json:"template_version"`
	Turns           []Turn         `json:"turns"`
	Labels          map[string]int `json:"labels"`
	LabelSource     string         `json:"label_source"`
}

// Manifest は書き出し1回ぶんの説明。
type Manifest struct {
	FormatVersion int       `json:"format_version"`
	RunID         string    `json:"run_id"`
	ExportedAt    time.Time `json:"exported_at"`
	LabelSource   string    `json:"label_source"`
	LabelNote     string    `json:"label_note"`
	RubricKeys    []string  `json:"rubric_keys"`
	MaskingTokens []string  `json:"masking_tokens"`
	StudentCount  int       `json:"student_count"`
	SessionCount  int       `json:"session_count"`
	Files         []string  `json:"files"`
	// FileSessions はファイルごとの面接の数。同意の取り消し・退会でファイルを消すときに、
	// 学生のファイルを読まずに session_count を直すために使う（sweep.go）。
	FileSessions map[string]int `json:"file_sessions"`
}

// Result は書き出しの結果。
type Result struct {
	RunID       string
	Destination string
	Students    int
	Sessions    int
	// SkippedSessions はラベルが壊れている・学生の回答が無いなどで書き出さなかった面接の数。
	SkippedSessions int
}

// Job は学習データの書き出しジョブ。
type Job struct {
	cfg    Config
	store  Store
	writer ObjectWriter
	now    func() time.Time
	// runSuffix は実行 ID の末尾に付けるランダムな値。テストで差し替える。
	runSuffix func() string
}

// randomRunSuffix は実行 ID の末尾に付ける 8 桁の16進。同じ秒に二重に起動しても（スケジューラの重複実行・
// 即時のリトライなど）出力先が衝突し、互いのファイルやマニフェストを上書きしないようにする（#2325 のレビュー）。
func randomRunSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand が失敗するのは OS の乱数源が使えないときだけ。時刻のナノ秒で代える（衝突はまず起きない）。
		return fmt.Sprintf("%08x", uint32(time.Now().UnixNano()))
	}
	return hex.EncodeToString(b[:])
}

// NewJob はジョブを作る。設定が不正ならエラーにする（ジョブを動かさない）。
func NewJob(cfg Config, store Store, writer ObjectWriter) (*Job, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if store == nil || writer == nil {
		return nil, errors.New("scorerexport: store と writer は必須です")
	}
	return &Job{cfg: cfg, store: store, writer: writer, now: time.Now, runSuffix: randomRunSuffix}, nil
}

// Run は書き出しを1回行い、監査ログに結果を残す。
// 監査ログを作れなければ何も書き出さない（記録の無い書き出しを作らないため）。
func (j *Job) Run(ctx context.Context) (Result, error) {
	startedAt := j.now()
	runID := startedAt.UTC().Format("20060102T150405Z") + "-" + j.runSuffix()
	runPrefix := j.cfg.Prefix + "/runs/" + runID + "/"
	res := Result{RunID: runID, Destination: "s3://" + j.cfg.Bucket + "/" + runPrefix}

	auditID, err := j.store.StartTrainingExportRun(ctx, res.Destination, startedAt)
	if err != nil {
		return res, fmt.Errorf("scorerexport: 監査ログを作れませんでした: %w", err)
	}

	runErr := j.export(ctx, runPrefix, runID, startedAt, &res)

	status, errMsg := models.TrainingScorerExportSucceeded, ""
	if runErr != nil {
		status, errMsg = models.TrainingScorerExportFailed, redactHashes(runErr.Error())
	}
	// 呼び出し側の ctx が取り消されて失敗した回も「失敗」として残すため、取り消しを外して書く。
	if err := j.store.FinishTrainingExportRun(context.WithoutCancel(ctx), auditID, status,
		res.Students, res.Sessions, errMsg, j.now()); err != nil {
		return res, errors.Join(runErr, fmt.Errorf("scorerexport: 監査ログに結果を書けませんでした: %w", err))
	}
	return res, runErr
}

// hashPattern は hash の出力（HMAC-SHA256 の16進64桁）。
var hashPattern = regexp.MustCompile(`[0-9a-f]{64}`)

// redactHashes は監査ログに残すエラーから subject を伏せる。S3 の AccessDenied などは
// エラーにオブジェクトのキー（subject を含む）を入れて返す。監査ログは退会のパージの対象にしないので、
// 利用者に結びつく値を残さない。
func redactHashes(msg string) string {
	return hashPattern.ReplaceAllString(msg, "<subject>")
}

func (j *Job) export(ctx context.Context, runPrefix, runID string, startedAt time.Time, res *Result) error {
	terms, err := j.store.ListTrainingMaskingTerms(ctx)
	if err != nil {
		return fmt.Errorf("scorerexport: マスキングの辞書の名称を取得できませんでした: %w", err)
	}
	subjects, err := j.store.ListTrainingSubjects(ctx)
	if err != nil {
		return fmt.Errorf("scorerexport: 対象の学生を取得できませんでした: %w", err)
	}

	var files []string
	fileSessions := map[string]int{}
	for _, subj := range subjects {
		lines, skipped, err := j.subjectLines(ctx, subj, terms)
		res.SkippedSessions += skipped
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			continue
		}
		// 取り出している間に同意を取り消した学生を書かないよう、書く直前に判定し直す。
		ok, err := j.store.IsTrainingEligible(ctx, subj.UserID)
		if err != nil {
			return fmt.Errorf("scorerexport: 学習に使ってよいかを判定できませんでした: %w", err)
		}
		if !ok {
			continue
		}
		key := runPrefix + "subjects/" + j.hash("user", subj.UserID) + ".jsonl"
		if err := j.writer.PutObject(ctx, key, bytes.Join(lines, nil), "application/x-ndjson"); err != nil {
			return fmt.Errorf("scorerexport: 書き込みに失敗しました: %w", err)
		}
		// 書く直前の判定と書き込みの間に同意を取り消した（または退会した）学生のファイルを残さないよう、
		// 書いた後にもう一度判定し、外れていれば消す（#2325 のレビュー）。消せなければ実行を失敗にする。
		// それでも残る「判定し直した後の取り消し」は、次の Sweeper.Sweep（次の書き出しの最初・定期実行）と
		// 退会のパージ（Sweeper.DeleteUser）が拾う。
		ok, err = j.store.IsTrainingEligible(ctx, subj.UserID)
		if err != nil || !ok {
			if delErr := j.writer.DeleteObject(ctx, key); delErr != nil {
				return fmt.Errorf("scorerexport: 同意を取り消した学生のファイルを消せませんでした: %w", delErr)
			}
			if err != nil {
				return fmt.Errorf("scorerexport: 学習に使ってよいかを判定できませんでした: %w", err)
			}
			continue
		}
		rel := strings.TrimPrefix(key, runPrefix)
		files = append(files, rel)
		fileSessions[rel] = len(lines)
		res.Students++
		res.Sessions += len(lines)
	}

	manifest := Manifest{
		FormatVersion: FormatVersion,
		RunID:         runID,
		ExportedAt:    startedAt.UTC(),
		LabelSource:   LabelSource,
		LabelNote:     labelNote,
		RubricKeys:    interview.RubricKeys(),
		MaskingTokens: maskingTokens(),
		StudentCount:  res.Students,
		SessionCount:  res.Sessions,
		Files:         files,
		FileSessions:  fileSessions,
	}
	if manifest.Files == nil {
		manifest.Files = []string{}
	}
	body, err := marshalJSON(manifest, "  ")
	if err != nil {
		return fmt.Errorf("scorerexport: マニフェストを作れませんでした: %w", err)
	}
	if err := j.writer.PutObject(ctx, runPrefix+"manifest.json", body, "application/json"); err != nil {
		return fmt.Errorf("scorerexport: マニフェストの書き込みに失敗しました: %w", err)
	}
	return nil
}

// subjectLines は学生1人ぶんの JSONL の行（末尾に改行つき）を作る。
func (j *Job) subjectLines(ctx context.Context, subj repositories.TrainingSubject, terms repositories.TrainingMaskingTerms) ([][]byte, int, error) {
	sessions, err := j.store.ListTrainingSessions(ctx, subj.UserID)
	if err != nil {
		return nil, 0, fmt.Errorf("scorerexport: 面接を取得できませんでした: %w", err)
	}
	if len(sessions) == 0 {
		return nil, 0, nil
	}
	masker, err := newMasker(subj, terms)
	if err != nil {
		return nil, 0, err
	}

	subject := j.hash("user", subj.UserID)
	var lines [][]byte
	skipped := 0
	for _, s := range sessions {
		labels, ok := parseLabels(s.ScoresJSON)
		if !ok {
			skipped++
			continue
		}
		utts, err := j.store.ListTrainingUtterances(ctx, s.SessionID)
		if err != nil {
			return nil, skipped, fmt.Errorf("scorerexport: 発話を取得できませんでした: %w", err)
		}
		turns := buildTurns(utts, masker)
		if len(turns) == 0 {
			skipped++
			continue
		}
		line, err := marshalJSON(Sample{
			FormatVersion:   FormatVersion,
			SampleID:        j.hash("session", s.SessionID),
			Subject:         subject,
			TemplateVersion: s.TemplateVersion,
			Turns:           turns,
			Labels:          labels,
			LabelSource:     LabelSource,
		}, "")
		if err != nil {
			return nil, skipped, fmt.Errorf("scorerexport: 行を作れませんでした: %w", err)
		}
		lines = append(lines, line)
	}
	return lines, skipped, nil
}

// hash は ID を秘密鍵の HMAC-SHA256 にする。kind で利用者と面接を分け、同じ数値の ID が同じ値にならないようにする。
// 書き方は管理者トークンの署名（middleware.adminTokenSignature）に合わせた。
func (j *Job) hash(kind string, id uint) string {
	return subjectHash(j.cfg.HMACKey, kind, id)
}

// subjectHash は ID を秘密鍵の HMAC-SHA256 の16進にする。書き出し（Job）と削除（Sweeper）で同じ値を作るため、1か所にする。
func subjectHash(key []byte, kind string, id uint) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s:%d", kind, id)
	return hex.EncodeToString(mac.Sum(nil))
}

// keyKnownEmail は、その学生のメールアドレスを辞書で照合する項目。
// 正規表現（masking.KeyEmail）は「taro@example」のようにトップレベルドメインが欠けた書き方を拾わないため、
// 本人のアドレスとローカル部（「@」より前）は辞書でも置き換える。
const keyKnownEmail = "known_email"

// maskingRules は書き出しで使う項目。DefaultRules に、本人のメールアドレスの辞書を足す。
func maskingRules() []masking.Rule {
	return append(masking.DefaultRules(), masking.Rule{
		Key: keyKnownEmail, Token: masking.TokenEmail, Method: masking.MethodDictionary,
	})
}

// InterviewShadowMask は、本番のシャドー採点が学習書き出しと同じ伏字を使うための読み込み。
// 学校名と企業名の一覧は 10 分キャッシュする。学生の氏名は毎回読む。
type InterviewShadowMask struct {
	users interface {
		GetUserByID(uint) (*models.User, error)
	}
	terms interface {
		ListTrainingMaskingTerms(context.Context) (repositories.TrainingMaskingTerms, error)
	}
	mu         sync.Mutex
	cached     repositories.TrainingMaskingTerms
	freshUntil time.Time
}

// NewInterviewShadowMask は user と学校・企業名の一覧から伏字関数を作る。
func NewInterviewShadowMask(
	users interface {
		GetUserByID(uint) (*models.User, error)
	},
	terms interface {
		ListTrainingMaskingTerms(context.Context) (repositories.TrainingMaskingTerms, error)
	},
) *InterviewShadowMask {
	return &InterviewShadowMask{users: users, terms: terms}
}

// ForUser は、その学生の辞書で伏字にする関数を返す。読めなければ採点してはならない。
func (m *InterviewShadowMask) ForUser(ctx context.Context, userID uint) (func(string) string, error) {
	if m == nil || m.users == nil || m.terms == nil {
		return nil, errors.New("scorerexport: 伏字の読み込みが未設定です")
	}
	u, err := m.users.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("scorerexport: 伏字にする学生が見つかりません")
	}
	terms, err := m.maskingTerms(ctx)
	if err != nil {
		return nil, err
	}
	return MaskText(repositories.TrainingSubject{
		UserID: u.ID, Name: u.Name, Email: u.Email, SchoolName: u.SchoolName,
	}, terms)
}

func (m *InterviewShadowMask) maskingTerms(ctx context.Context) (repositories.TrainingMaskingTerms, error) {
	m.mu.Lock()
	if time.Now().Before(m.freshUntil) {
		terms := m.cached
		m.mu.Unlock()
		return terms, nil
	}
	m.mu.Unlock()
	loaded, err := m.terms.ListTrainingMaskingTerms(ctx)
	if err != nil {
		return repositories.TrainingMaskingTerms{}, err
	}
	m.mu.Lock()
	m.cached = loaded
	m.freshUntil = time.Now().Add(10 * time.Minute)
	m.mu.Unlock()
	return loaded, nil
}

// MaskText は学習書き出しと同じ辞書で伏字にする関数を返す。本番の推論もこれを使う。
func MaskText(subj repositories.TrainingSubject, terms repositories.TrainingMaskingTerms) (func(string) string, error) {
	m, err := newMasker(subj, terms)
	if err != nil {
		return nil, err
	}
	return m.Mask, nil
}

// newMasker は学生1人ぶんの辞書で Masker を作る。
// 氏名は NameTerms で姓・名の揺れを足す。学校名は本人の自由記述も入れる（全員分の一覧にも入っているが、
// 一覧の取得の条件が変わっても本人の分は必ず入るようにするため）。
func newMasker(subj repositories.TrainingSubject, terms repositories.TrainingMaskingTerms) (*masking.Masker, error) {
	var emails []string
	if e := strings.TrimSpace(subj.Email); e != "" {
		emails = append(emails, e)
		if local, _, ok := strings.Cut(e, "@"); ok && local != "" {
			emails = append(emails, local)
		}
	}
	dict := masking.Dictionary{
		masking.KeyName:    masking.NameTerms(subj.Name),
		masking.KeySchool:  append(append([]string{}, terms.Schools...), subj.SchoolName),
		masking.KeyCompany: terms.Companies,
		keyKnownEmail:      emails,
	}
	m, err := masking.New(maskingRules(), dict)
	if err != nil {
		return nil, fmt.Errorf("scorerexport: マスキングの準備に失敗しました: %w", err)
	}
	return m, nil
}

// maskingTokens はマニフェストに書く、置き換えの記号の一覧（重複なし、定義の順）。
func maskingTokens() []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range maskingRules() {
		if !seen[r.Token] {
			seen[r.Token] = true
			out = append(out, r.Token)
		}
	}
	return out
}

// marshalJSON は v を JSON にする（末尾に改行つき。JSONL の1行にそのまま使える）。
// json.Marshal は「<」「>」を < にするので、置き換えの記号（<NAME> など）が読めるよう、
// HTML 向けのエスケープを切る。
func marshalJSON(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// parseLabels は LLM の採点を読み、ルーブリックの項目と値域を満たすものだけを返す。
// 欠けた・範囲外の採点を学習に入れると、モデルが誤った値を覚えるため。
func parseLabels(scoresJSON string) (map[string]int, bool) {
	var labels map[string]int
	if err := json.Unmarshal([]byte(scoresJSON), &labels); err != nil || labels == nil {
		return nil, false
	}
	if err := interview.ValidateRubricScores(labels); err != nil {
		return nil, false
	}
	return labels, true
}

// buildTurns は発話の並びを「AI の質問と学生の回答」の組にする。どちらもマスキングを通す。
//   - AI の発話が続いたら、まとめて1つの質問にする。
//   - 学生の発話が続いたら（音声認識で区切られた場合など）、まとめて1つの回答にする。
//   - 最初の質問より前の学生の発話は、質問を空にして残す。
//   - 回答の無い質問（面接の最後の締めの言葉など）は入れない。
//
// user・ai 以外の役割は入力に使わない。
func buildTurns(utts []repositories.TrainingUtterance, m *masking.Masker) []Turn {
	var turns []Turn
	var q []string
	answering := false
	for _, u := range utts {
		text := strings.TrimSpace(u.Text)
		if text == "" {
			continue
		}
		switch u.Role {
		case "ai":
			if answering {
				q, answering = nil, false
			}
			q = append(q, m.Mask(text))
		case "user":
			a := m.Mask(text)
			if answering {
				turns[len(turns)-1].A += "\n" + a
				continue
			}
			turns = append(turns, Turn{Q: strings.Join(q, "\n"), A: a})
			answering = true
		}
	}
	return turns
}

// RunFromEnv は CLI（cmd/training-export）の本体。環境変数から設定を読み、DB と出力先を用意して、
// 書き出し済みのデータの削除（Sweep）→ 書き出し、の順に1回行う。設定の確認を DB への接続より先に行う。
// Sweep に失敗したら書き出さない（取り消した学生のデータが残っている状態を、成功の書き出しで隠さないため）。
// cmd/ は CI のテストの対象外なので、配線をここに置いてテストする。
func RunFromEnv(
	ctx context.Context,
	getenv func(string) string,
	connect func() (Store, error),
	newObjects func(ctx context.Context, bucket string) (ObjectStore, error),
) (Result, SweepResult, error) {
	cfg, store, objects, err := setupFromEnv(ctx, getenv, connect, newObjects)
	if err != nil {
		return Result{}, SweepResult{}, err
	}
	sweeper, err := NewSweeper(cfg, store, objects)
	if err != nil {
		return Result{}, SweepResult{}, err
	}
	swept, err := sweeper.Sweep(ctx)
	if err != nil {
		return Result{}, swept, fmt.Errorf("scorerexport: 書き出し済みのデータを消せなかったので、書き出しを止めました: %w", err)
	}
	job, err := NewJob(cfg, store, objects)
	if err != nil {
		return Result{}, swept, err
	}
	res, err := job.Run(ctx)
	return res, swept, err
}

// SweepFromEnv は、書き出し済みのデータの削除（Sweep）だけを1回行う（cmd/training-export -sweep-only）。
// 同意の取り消し・退会を、書き出しの間隔より短い間隔（毎日など）で反映するために、定期的に動かす。
func SweepFromEnv(
	ctx context.Context,
	getenv func(string) string,
	connect func() (Store, error),
	newObjects func(ctx context.Context, bucket string) (ObjectStore, error),
) (SweepResult, error) {
	cfg, store, objects, err := setupFromEnv(ctx, getenv, connect, newObjects)
	if err != nil {
		return SweepResult{}, err
	}
	sweeper, err := NewSweeper(cfg, store, objects)
	if err != nil {
		return SweepResult{}, err
	}
	return sweeper.Sweep(ctx)
}

func setupFromEnv(
	ctx context.Context,
	getenv func(string) string,
	connect func() (Store, error),
	newObjects func(ctx context.Context, bucket string) (ObjectStore, error),
) (Config, Store, ObjectStore, error) {
	cfg, err := ConfigFromEnv(getenv)
	if err != nil {
		return Config{}, nil, nil, err
	}
	store, err := connect()
	if err != nil {
		return Config{}, nil, nil, fmt.Errorf("scorerexport: DB に接続できませんでした: %w", err)
	}
	objects, err := newObjects(ctx, cfg.Bucket)
	if err != nil {
		return Config{}, nil, nil, fmt.Errorf("scorerexport: 出力先を用意できませんでした: %w", err)
	}
	return cfg, store, objects, nil
}
