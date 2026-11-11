package prompts

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"Backend/internal/usagectx"
)

// プロンプトの A/B 割当（#1976）。
//
// 工程ごとに候補の版と比率を PROMPT_AB_<STAGE> で指定する（例: "v2:50,v3:50"）。
// 割当は利用者（または企業）ID とサーバー側の固定ソルトのハッシュで決めるので、
// 同じ ID は再起動やタスクをまたいでも同じ版になる。
//
// 候補にできるのは、今の版（ResolvePromptVersion）と、本文を登録した候補の版だけ。
// 本文を差し替えられない工程や、本文の無い版を指定した設定は無効にする。
// 割当だけを入れると同じ本文同士を比べることになり、実験として意味を持たないため。

// abSalt は割当のハッシュに混ぜる固定値。変えると全員の割当が入れ替わるので変えない。
const abSalt = "soc-ai-agent/prompt-ab/v1"

// maxABWeight は1つの候補の比率の上限。
const maxABWeight = 10000

// 本文の名前。1つの工程が複数の本文を使う場合（企業情報の抽出）は、すべてを同じ版で揃える。
const (
	BodyAnswerQuality        = "answer_quality"
	BodyMatchingReasonSystem = "matching_reason_system"
	BodyCompanyExtractSearch = "company_extract_search"
	BodyCompanyExtractFacts  = "company_extract_facts"
	subjectKindUser          = "user"
	subjectKindCompany       = "company"
)

// abBodies は A/B で本文を差し替えられる工程と、その工程が使う本文の名前。
// 質問生成は本文が多数のビルダーに分かれていて差し替える単位が無いので、対象外にする。
var abBodies = map[Stage][]string{
	StageScoreEvaluation:   {BodyAnswerQuality},
	StageMatchingReason:    {BodyMatchingReasonSystem},
	StageCompanyExtraction: {BodyCompanyExtractSearch, BodyCompanyExtractFacts},
}

var (
	candidateMu sync.RWMutex
	// candidateBodies は今の版以外の候補の本文。工程 → 版 → 本文の名前 → 本文。
	candidateBodies = map[Stage]map[string]map[string]string{}
)

// Subject は割当の単位。
type Subject struct {
	Kind string
	ID   uint
}

// UserSubject は利用者を単位にする。利用者を持つ経路（チャット、マッチング理由）で使う。
func UserSubject(id uint) Subject { return Subject{Kind: subjectKindUser, ID: id} }

// CompanySubject は企業を単位にする。利用者を持たない企業情報の抽出で使う。
func CompanySubject(id uint) Subject { return Subject{Kind: subjectKindCompany, ID: id} }

func (s Subject) valid() bool {
	return s.ID > 0 && (s.Kind == subjectKindUser || s.Kind == subjectKindCompany)
}

// RegisterCandidateBodies は候補の版の本文を登録する。工程が使う本文をすべて渡すこと。
// 候補の本文を足すときは、このパッケージのファイルの init から呼ぶ。
// 戻り値は登録を取り消す関数（テスト用）。
func RegisterCandidateBodies(stage Stage, version string, bodies map[string]string) (unregister func()) {
	candidateMu.Lock()
	defer candidateMu.Unlock()
	if candidateBodies[stage] == nil {
		candidateBodies[stage] = map[string]map[string]string{}
	}
	copied := make(map[string]string, len(bodies))
	for name, body := range bodies {
		copied[name] = body
	}
	candidateBodies[stage][version] = copied
	return func() {
		candidateMu.Lock()
		defer candidateMu.Unlock()
		delete(candidateBodies[stage], version)
	}
}

// Body は版に対応する本文を返す。今の版、または候補の本文が無い版なら current を返す。
// current は呼び出し側がコードに持っている今の本文。
func Body(stage Stage, version, name, current string) string {
	if version == "" || version == ResolvePromptVersion(stage) {
		return current
	}
	candidateMu.RLock()
	defer candidateMu.RUnlock()
	if body, ok := candidateBodies[stage][version][name]; ok {
		return body
	}
	return current
}

// hasBodies は版が工程のすべての本文を持つかを返す。今の版は常に持つ。
func hasBodies(stage Stage, version string) bool {
	names, ok := abBodies[stage]
	if !ok {
		return false
	}
	if version == ResolvePromptVersion(stage) {
		return true
	}
	candidateMu.RLock()
	defer candidateMu.RUnlock()
	bodies := candidateBodies[stage][version]
	for _, name := range names {
		if _, ok := bodies[name]; !ok {
			return false
		}
	}
	return true
}

type abArm struct {
	version string
	weight  uint64
}

// abArms は工程の A/B 設定を読む。無効な設定なら nil を返し、一度だけ警告を出す。
func abArms(stage Stage) []abArm {
	raw := strings.TrimSpace(os.Getenv("PROMPT_AB_" + strings.ToUpper(string(stage))))
	if raw == "" {
		return nil
	}
	if _, ok := abBodies[stage]; !ok {
		warnABOnce(stage, raw, "stage has no selectable body")
		return nil
	}
	var arms []abArm
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		version, weightText, ok := strings.Cut(strings.TrimSpace(part), ":")
		version = strings.TrimSpace(version)
		weight, err := strconv.ParseUint(strings.TrimSpace(weightText), 10, 64)
		switch {
		case !ok || err != nil || weight == 0 || weight > maxABWeight:
			warnABOnce(stage, raw, "invalid weight")
			return nil
		case !ValidPromptVersion(version):
			warnABOnce(stage, raw, "invalid version")
			return nil
		case seen[version]:
			warnABOnce(stage, raw, "duplicate version")
			return nil
		case !hasBodies(stage, version):
			warnABOnce(stage, raw, "version has no registered body")
			return nil
		}
		seen[version] = true
		arms = append(arms, abArm{version: version, weight: weight})
	}
	if len(arms) < 2 {
		warnABOnce(stage, raw, "needs at least two versions")
		return nil
	}
	return arms
}

// assignArm は ID のハッシュから候補を選ぶ。比率は weight の比になる。
func assignArm(stage Stage, subject Subject, arms []abArm) string {
	var total uint64
	for _, arm := range arms {
		total += arm.weight
	}
	sum := sha256.Sum256([]byte(abSalt + "\x00" + string(stage) + "\x00" + subject.Kind + "\x00" + strconv.FormatUint(uint64(subject.ID), 10)))
	bucket := binary.BigEndian.Uint64(sum[:8]) % total
	for _, arm := range arms {
		if bucket < arm.weight {
			return arm.version
		}
		bucket -= arm.weight
	}
	return arms[len(arms)-1].version
}

// AssignPromptVersion は工程と割当の単位から版を決める。A/B の対象なら group に
// 割り当てた版を返し、対象外なら今の版と空の group を返す。
func AssignPromptVersion(stage Stage, subject Subject) (version, group string) {
	if subject.valid() {
		if arms := abArms(stage); arms != nil {
			version = assignArm(stage, subject, arms)
			return version, version
		}
	}
	return ResolvePromptVersion(stage), ""
}

// WithPromptVersionFor は割り当てた版を利用量記録用 context に載せる。
// group は api_call_logs.prompt_ab_group に残り、A/B の比較に使う。
func WithPromptVersionFor(ctx context.Context, stage Stage, subject Subject) (context.Context, string) {
	version, group := AssignPromptVersion(stage, subject)
	return usagectx.WithPromptAssignment(ctx, string(stage), version, group), version
}

func warnABOnce(stage Stage, raw, reason string) {
	key := "ab:" + string(stage) + ":" + reason + ":" + raw
	if _, loaded := warnedPromptSettings.LoadOrStore(key, struct{}{}); !loaded {
		slog.Warn("プロンプトの A/B 設定を無視します", "stage", stage, "reason", reason)
	}
}
