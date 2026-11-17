package scorerexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"Backend/internal/models"
)

// ErrObjectNotFound は、読もうとしたオブジェクトが無いことを表す（ObjectStore.GetObject が返す）。
var ErrObjectNotFound = errors.New("scorerexport: オブジェクトがありません")

// ObjectStore は、書き出し済みのデータを消すときに使う出力先の操作。本番は S3（S3Writer）。
//
// 学生のファイル（subjects/*.jsonl）は読まない。消すのに必要なのは、キーの一覧・マニフェストの読み書き・削除だけ
// （マニフェストには個人のデータが無い）。そのため、パージを行うサーバーに学習データを読む権限を付けなくてよい。
type ObjectStore interface {
	ObjectWriter
	// ListKeys は prefix で始まるキーをすべて返す。
	ListKeys(ctx context.Context, prefix string) ([]string, error)
	// GetObject は key の中身を返す。無ければ ErrObjectNotFound。
	GetObject(ctx context.Context, key string) ([]byte, error)
}

// SweepStore は、消すときに使う DB の操作。実装は repositories.TrainingScorerExportRepository。
type SweepStore interface {
	// ListTrainingEligibleUserIDs は、いま学習に使ってよい学生の利用者 ID を返す（trainingEligibleUsers と同じ条件）。
	ListTrainingEligibleUserIDs(ctx context.Context) ([]uint, error)
	StartTrainingSweepRun(ctx context.Context, destination string, startedAt time.Time) (uint, error)
	FinishTrainingSweepRun(ctx context.Context, id uint, status string, deletedFiles int, errMsg string, finishedAt time.Time) error
}

// Sweeper は、同意を取り消した学生と退会した学生の、書き出し済みのデータを消す（#2168）。
//
//   - Sweep：いま学習に使ってよい学生「以外」のファイルを、すべての回から消す。書き出しのたびに最初に行うほか、
//     単独でも定期的に動かす（cmd/training-export -sweep-only）。同意の取り消し・退会はここで拾う。
//   - DeleteUser：1人ぶんのファイルを、すべての回から消す。退会のパージ（hardDeleteUserData の後）から呼ぶ。
//
// どちらも、ファイルを消す前に、その回のマニフェストから消すファイルを外して書き直す。
// 順番を逆にすると、マニフェストの書き直しに失敗したときに「載っているのに無いファイル」が残り、
// 学習スクリプトがその回を読めなくなる。マニフェストを先に直しておけば、ファイルの削除に失敗しても、
// 次の Sweep がそのファイルを見つけて消し直せる。
type Sweeper struct {
	cfg     Config
	store   SweepStore
	objects ObjectStore
	now     func() time.Time
}

// NewSweeper は Sweeper を作る。設定が不正ならエラーにする。store は Sweep を使わない
// （DeleteUser だけ使う）ときは nil でよい。
func NewSweeper(cfg Config, store SweepStore, objects ObjectStore) (*Sweeper, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if objects == nil {
		return nil, errors.New("scorerexport: objects は必須です")
	}
	return &Sweeper{cfg: cfg, store: store, objects: objects, now: time.Now}, nil
}

// SweepResult は Sweep の結果。
type SweepResult struct {
	// DeletedFiles は消した学生のファイルの数（回ごとに数える。同じ学生が3回ぶんにあれば3）。
	DeletedFiles int
	// UpdatedManifests は書き直したマニフェストの数。
	UpdatedManifests int
}

// Sweep は、いま学習に使ってよい学生以外のファイルを、すべての回から消し、監査ログに残す。
// 対象の学生の一覧を取れなければ、何も消さずにエラーにする（一覧が空のまま進めて、全員分を消さないため）。
func (s *Sweeper) Sweep(ctx context.Context) (SweepResult, error) {
	if s.store == nil {
		return SweepResult{}, errors.New("scorerexport: Sweep には store が必要です")
	}
	startedAt := s.now()
	destination := "s3://" + s.cfg.Bucket + "/" + s.runsPrefix()
	auditID, err := s.store.StartTrainingSweepRun(ctx, destination, startedAt)
	if err != nil {
		return SweepResult{}, fmt.Errorf("scorerexport: 監査ログを作れませんでした: %w", err)
	}

	res, runErr := s.sweep(ctx)

	status, errMsg := models.TrainingScorerExportSucceeded, ""
	if runErr != nil {
		status, errMsg = models.TrainingScorerExportFailed, redactHashes(runErr.Error())
	}
	if err := s.store.FinishTrainingSweepRun(context.WithoutCancel(ctx), auditID, status, res.DeletedFiles, errMsg, s.now()); err != nil {
		return res, errors.Join(runErr, fmt.Errorf("scorerexport: 監査ログに結果を書けませんでした: %w", err))
	}
	return res, runErr
}

func (s *Sweeper) sweep(ctx context.Context) (SweepResult, error) {
	ids, err := s.store.ListTrainingEligibleUserIDs(ctx)
	if err != nil {
		return SweepResult{}, fmt.Errorf("scorerexport: 学習に使ってよい学生を取得できませんでした: %w", err)
	}
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[subjectHash(s.cfg.HMACKey, "user", id)] = true
	}
	return s.remove(ctx, func(subject string) bool { return !keep[subject] })
}

// DeleteUser は1人ぶんのファイルを、すべての回から消し、消した数を返す。
// 利用者の行が DB から消えた後（パージの後）でも、利用者 ID から subject を作れるので消せる。
// エラーは subject を伏せて返す（呼び出し側の監査ログに残るため）。
func (s *Sweeper) DeleteUser(ctx context.Context, userID uint) (int, error) {
	target := subjectHash(s.cfg.HMACKey, "user", userID)
	res, err := s.remove(ctx, func(subject string) bool { return subject == target })
	if err != nil {
		return res.DeletedFiles, errors.New(redactHashes(err.Error()))
	}
	return res.DeletedFiles, nil
}

// NewUserDataDeleterFromEnv は、退会のパージから使う Sweeper を環境変数から作る（サーバーの配線用）。
// 書き出しの設定（EnvHMACKey・EnvDestination）がどちらも無ければ (nil, nil) を返す（書き出しも動かないので消すものが無い）。
// 片方だけ・形が違うなどの誤りはエラーにする（パージで消し損ねるので、起動時に分かるようにする）。
func NewUserDataDeleterFromEnv(
	ctx context.Context,
	getenv func(string) string,
	newObjects func(ctx context.Context, bucket string) (ObjectStore, error),
) (*Sweeper, error) {
	if strings.TrimSpace(getenv(EnvHMACKey)) == "" && strings.TrimSpace(getenv(EnvDestination)) == "" {
		return nil, nil
	}
	cfg, err := ConfigFromEnv(getenv)
	if err != nil {
		return nil, err
	}
	objects, err := newObjects(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("scorerexport: 出力先を用意できませんでした: %w", err)
	}
	return NewSweeper(cfg, nil, objects)
}

func (s *Sweeper) runsPrefix() string { return s.cfg.Prefix + "/runs/" }

// subjectKeyPattern は学生のファイルのキー（runs/ より後ろ）。<実行ID>/subjects/<subject>.jsonl。
var subjectKeyPattern = regexp.MustCompile(`^([^/]+)/subjects/([0-9a-f]{64})\.jsonl$`)

// remove は、drop が true を返す subject のファイルを、すべての回から消す。
// 1つの回で失敗しても、ほかの回は続け、最後にまとめてエラーを返す。
func (s *Sweeper) remove(ctx context.Context, drop func(subject string) bool) (SweepResult, error) {
	var res SweepResult
	runs := s.runsPrefix()
	keys, err := s.objects.ListKeys(ctx, runs)
	if err != nil {
		return res, fmt.Errorf("scorerexport: 書き出し済みのファイルの一覧を取得できませんでした: %w", err)
	}
	// 実行 ID ごとに、消すファイル（runs/<実行ID>/ からの相対パス）をまとめる。
	byRun := map[string][]string{}
	for _, key := range keys {
		m := subjectKeyPattern.FindStringSubmatch(strings.TrimPrefix(key, runs))
		if m == nil || !drop(m[2]) {
			continue
		}
		byRun[m[1]] = append(byRun[m[1]], "subjects/"+m[2]+".jsonl")
	}
	runIDs := make([]string, 0, len(byRun))
	for id := range byRun {
		runIDs = append(runIDs, id)
	}
	sort.Strings(runIDs)

	var errs []error
	for _, runID := range runIDs {
		files := byRun[runID]
		sort.Strings(files)
		runPrefix := runs + runID + "/"
		updated, err := s.dropFromManifest(ctx, runPrefix, files)
		if err != nil {
			// マニフェストを直せなかった回のファイルは消さない（載っているのに無いファイルを作らないため）。
			errs = append(errs, fmt.Errorf("回 %s: %w", runID, err))
			continue
		}
		if updated {
			res.UpdatedManifests++
		}
		for _, f := range files {
			if err := s.objects.DeleteObject(ctx, runPrefix+f); err != nil {
				errs = append(errs, fmt.Errorf("回 %s: ファイルを消せませんでした: %w", runID, err))
				continue
			}
			res.DeletedFiles++
		}
	}
	return res, errors.Join(errs...)
}

// dropFromManifest は、その回のマニフェストから files を外し、件数を直して書き直す。
// マニフェストが無い回（書き出しの途中・途中で落ちた回）は、直すものが無いので何もしない。
// 書き直したら true を返す。
func (s *Sweeper) dropFromManifest(ctx context.Context, runPrefix string, files []string) (bool, error) {
	body, err := s.objects.GetObject(ctx, runPrefix+"manifest.json")
	if errors.Is(err, ErrObjectNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("マニフェストを読めませんでした: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return false, fmt.Errorf("マニフェストの形が違います: %w", err)
	}
	drop := make(map[string]bool, len(files))
	for _, f := range files {
		drop[f] = true
	}
	kept := make([]string, 0, len(m.Files))
	changed := false
	for _, f := range m.Files {
		if !drop[f] {
			kept = append(kept, f)
			continue
		}
		changed = true
		m.StudentCount--
		m.SessionCount -= m.FileSessions[f]
		delete(m.FileSessions, f)
	}
	if !changed {
		return false, nil
	}
	m.Files = kept
	m.StudentCount = max(m.StudentCount, 0)
	m.SessionCount = max(m.SessionCount, 0)
	out, err := marshalJSON(m, "  ")
	if err != nil {
		return false, fmt.Errorf("マニフェストを作れませんでした: %w", err)
	}
	if err := s.objects.PutObject(ctx, runPrefix+"manifest.json", out, "application/json"); err != nil {
		return false, fmt.Errorf("マニフェストを書き直せませんでした: %w", err)
	}
	return true, nil
}
