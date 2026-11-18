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
	// RepairedEntries は、マニフェストに載っているのに実在しなかったので外したファイルの数。
	// 削除どうしが同時に走ってマニフェストの書き直しが重なった、書き出しの途中で消した、などのずれを、ここで直す。
	RepairedEntries int
	// UncountedManifests は、file_sessions を持たない（この仕組みより前に書いた）ため、
	// ファイルを外しても session_count を直せなかったマニフェストの数。session_count はそのまま残る（多めになる）。
	UncountedManifests int
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
// あわせて、マニフェストのある回は、載っているのに実在しないファイルをマニフェストから外す（照合）。
// マニフェストの書き直しは排他していない（S3 に読み書きのロックが無い）ので、削除どうしが同時に走ると、
// 後から書いたほうが先に外したファイルを戻すことがある。そのファイルは既に消えていてキーの一覧に出ないため、
// 消す対象からだけでは直せない。照合を毎回行い、次の削除で直す。
// 1つの回で失敗しても、ほかの回は続け、最後にまとめてエラーを返す。
func (s *Sweeper) remove(ctx context.Context, drop func(subject string) bool) (SweepResult, error) {
	var res SweepResult
	runs := s.runsPrefix()
	keys, err := s.objects.ListKeys(ctx, runs)
	if err != nil {
		return res, fmt.Errorf("scorerexport: 書き出し済みのファイルの一覧を取得できませんでした: %w", err)
	}
	// 実行 ID ごとに、消すファイル（runs/<実行ID>/ からの相対パス）と、実在するキーをまとめる。
	byRun := map[string][]string{}
	present := map[string]map[string]bool{}
	hasManifest := map[string]bool{}
	for _, key := range keys {
		rel := strings.TrimPrefix(key, runs)
		runID, file, ok := strings.Cut(rel, "/")
		if !ok || runID == "" {
			continue
		}
		if present[runID] == nil {
			present[runID] = map[string]bool{}
		}
		present[runID][file] = true
		if file == "manifest.json" {
			hasManifest[runID] = true
			continue
		}
		m := subjectKeyPattern.FindStringSubmatch(rel)
		if m == nil || !drop(m[2]) {
			continue
		}
		byRun[runID] = append(byRun[runID], file)
	}
	runIDs := make([]string, 0, len(byRun)+len(hasManifest))
	for id := range present {
		if len(byRun[id]) > 0 || hasManifest[id] {
			runIDs = append(runIDs, id)
		}
	}
	sort.Strings(runIDs)

	var errs []error
	for _, runID := range runIDs {
		files := byRun[runID]
		sort.Strings(files)
		runPrefix := runs + runID + "/"
		if hasManifest[runID] {
			fix, err := s.dropFromManifest(ctx, runPrefix, files, present[runID])
			if err != nil {
				// マニフェストを直せなかった回のファイルは消さない（載っているのに無いファイルを作らないため）。
				errs = append(errs, fmt.Errorf("回 %s: %w", runID, err))
				continue
			}
			if fix.updated {
				res.UpdatedManifests++
			}
			res.RepairedEntries += fix.repaired
			if fix.uncounted {
				res.UncountedManifests++
			}
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

// manifestFix は dropFromManifest の結果。
type manifestFix struct {
	updated   bool // 書き直した
	repaired  int  // 載っているのに実在しなかったので外した数
	uncounted bool // file_sessions が無く、session_count を直せなかった
}

// dropFromManifest は、その回のマニフェストから files と、載っているのに実在しないファイルを外し、件数を直して書き直す。
// present は、キーの一覧で見つかったその回のキー（runs/<実行ID>/ からの相対パス）。
func (s *Sweeper) dropFromManifest(ctx context.Context, runPrefix string, files []string, present map[string]bool) (manifestFix, error) {
	var fix manifestFix
	body, err := s.objects.GetObject(ctx, runPrefix+"manifest.json")
	if errors.Is(err, ErrObjectNotFound) {
		// 一覧の後に消えた（直すものが無い）。
		return fix, nil
	}
	if err != nil {
		return fix, fmt.Errorf("マニフェストを読めませんでした: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return fix, fmt.Errorf("マニフェストの形が違います: %w", err)
	}
	drop := make(map[string]bool, len(files))
	for _, f := range files {
		drop[f] = true
	}
	missing, err := s.missingFiles(ctx, runPrefix, m.Files, drop, present)
	if err != nil {
		return fix, err
	}
	kept := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		if !drop[f] && !missing[f] {
			kept = append(kept, f)
			continue
		}
		if missing[f] {
			fix.repaired++
		}
		m.StudentCount--
		if m.FileSessions == nil {
			// この仕組みより前に書いたマニフェスト。ファイルごとの件数が分からないので、session_count は直さない。
			fix.uncounted = true
		} else {
			m.SessionCount -= m.FileSessions[f]
			delete(m.FileSessions, f)
		}
	}
	if len(kept) == len(m.Files) {
		return manifestFix{}, nil
	}
	m.Files = kept
	m.StudentCount = max(m.StudentCount, 0)
	m.SessionCount = max(m.SessionCount, 0)
	out, err := marshalJSON(m, "  ")
	if err != nil {
		return manifestFix{}, fmt.Errorf("マニフェストを作れませんでした: %w", err)
	}
	if err := s.objects.PutObject(ctx, runPrefix+"manifest.json", out, "application/json"); err != nil {
		return manifestFix{}, fmt.Errorf("マニフェストを書き直せませんでした: %w", err)
	}
	fix.updated = true
	return fix, nil
}

// missingFiles は、マニフェストに載っているファイルのうち、実在しないもの（drop 以外）を返す。
// 最初のキーの一覧（present）は、マニフェストを読む前に取ったもの。その間に書き出しがファイルとマニフェストを
// 書いていると、実在するファイルを「無い」と見てしまうので、無いものがあればその回のキーを一覧し直して確かめる
// （書き出しはファイルを先に、マニフェストを最後に書くので、マニフェストを読んだ後の一覧には載っている）。
func (s *Sweeper) missingFiles(ctx context.Context, runPrefix string, files []string, drop, present map[string]bool) (map[string]bool, error) {
	var candidates []string
	for _, f := range files {
		if !drop[f] && !present[f] {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	keys, err := s.objects.ListKeys(ctx, runPrefix)
	if err != nil {
		return nil, fmt.Errorf("マニフェストとの照合のためのファイルの一覧を取得できませんでした: %w", err)
	}
	now := make(map[string]bool, len(keys))
	for _, k := range keys {
		now[strings.TrimPrefix(k, runPrefix)] = true
	}
	missing := map[string]bool{}
	for _, f := range candidates {
		if !now[f] {
			missing[f] = true
		}
	}
	return missing, nil
}
