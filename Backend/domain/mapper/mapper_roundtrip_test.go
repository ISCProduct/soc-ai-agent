package mapper_test

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/internal/models"
)

// mapper が、model と entity の両方にある項目を、取りこぼさずに運ぶことを検査する。
//
// model・entity・mapper の3か所を手で揃えているので、項目を足したときに mapper を直し忘れると、
// その項目は変換のたびに黙って落ちる(#1155、#2180 で起きた)。UpdateUser のように
// 行全体を保存する経路では、落ちた値がそのまま DB に書き戻される。
//
// 検査は3つ。
//  1. ToEntity: 両方にある項目が、そのまま entity に渡ること
//  2. FromEntity: 両方にある項目が、そのまま model に戻ること(FromEntity がある型だけ)
//  3. 片方にしかない項目の一覧が、下の許可リストと一致すること
//     (項目を足したら、このテストが落ちる。取りこぼしか意図した差かを、理由つきで決めさせる)
//
// 許可リストは「現在の差」の記録で、全てが設計上望ましいという意味ではない。

type mapperCase struct {
	name       string
	model      reflect.Type
	entity     reflect.Type
	toEntity   func(m any) any
	fromEntity func(e any) any // nil なら、片方向(ToEntity のみ)の型

	// model にだけある項目 → 理由。entity には無いので、entity 経由では読み書きされない。
	modelOnly map[string]string
	// 両方にあるが型が違い、そのままは比べられない項目 → 理由。
	typeDiffers map[string]string
	// 両方にあるが、FromEntity が意図して書き戻さない項目 → 理由。
	notFromEntity map[string]string
}

const modelOnlyReason = "entity に無い(モデル専用)。entity 経由では読み書きされない"

func only(reason string, names ...string) map[string]string {
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[n] = reason
	}
	return m
}

func typ[T any]() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }

var mapperCases = []mapperCase{
	{
		name: "User", model: typ[models.User](), entity: typ[entity.User](),
		toEntity:   func(m any) any { return mapper.UserToEntity(m.(*models.User)) },
		fromEntity: func(e any) any { return mapper.UserFromEntity(e.(*entity.User)) },
	},
	{
		// ToEntity のみ。企業の書き込みは models.Company を直接使う。
		name: "Company", model: typ[models.Company](), entity: typ[entity.Company](),
		toEntity: func(m any) any { return mapper.CompanyToEntity(m.(*models.Company)) },
		modelOnly: only(modelOnlyReason,
			"CicdTools", "DeletedAt", "GBizLastSyncedAt", "GBizSyncMessage", "GBizSyncStatus",
			"InfoFetchedAt", "InfoPromptVersion", "InfraStack", "IsGuestEntry", "JobsFetchedAt",
			"LastFetchConfidence", "LastModelUsed", "NameReading", "RelationsFetchedAt", "TechFetchedAt"),
	},
	{
		name: "UserApplicationStatus", model: typ[models.UserApplicationStatus](), entity: typ[entity.UserApplicationStatus](),
		toEntity:      func(m any) any { return mapper.UserApplicationStatusToEntity(m.(*models.UserApplicationStatus)) },
		fromEntity:    func(e any) any { return mapper.UserApplicationStatusFromEntity(e.(*entity.UserApplicationStatus)) },
		modelOnly:     only(modelOnlyReason, "ActiveDedupKey", "User"),
		typeDiffers:   only("関連(Preload)。entity は *Company、model は値。ToEntity が CompanyToEntity で変換する", "Company"),
		notFromEntity: only("関連は書き戻さない。保存には外部キー(CompanyID)を使う", "Company"),
	},
	{
		name: "UserCompanyMatch", model: typ[models.UserCompanyMatch](), entity: typ[entity.UserCompanyMatch](),
		toEntity:   func(m any) any { return mapper.UserCompanyMatchToEntity(m.(*models.UserCompanyMatch)) },
		fromEntity: func(e any) any { return mapper.UserCompanyMatchFromEntity(e.(*entity.UserCompanyMatch)) },
		modelOnly: map[string]string{
			"User":          modelOnlyReason,
			"JobPosition":   modelOnlyReason,
			"JobPositionID": modelOnlyReason + "。注意: upsert(user_company_match_repository.go の upsertAssignments)は job_position_id を更新対象にするが、UserCompanyMatchFromEntity は書き戻さない",
		},
		typeDiffers:   only("関連(Preload)。entity は *Company、model は値。ToEntity が CompanyToEntity で変換する", "Company"),
		notFromEntity: only("関連は書き戻さない。保存には外部キー(CompanyID)を使う", "Company"),
	},
	{
		// ToEntity のみ。重みの書き込みは models.UserWeightScore を直接使う。
		name: "UserWeightScore", model: typ[models.UserWeightScore](), entity: typ[entity.UserWeightScore](),
		toEntity:  func(m any) any { return mapper.UserWeightScoreToEntity(m.(*models.UserWeightScore)) },
		modelOnly: only(modelOnlyReason, "OrganizationID", "Source"),
	},
	{
		// ToEntity のみ。フェーズは参照だけ。
		name: "AnalysisPhase", model: typ[models.AnalysisPhase](), entity: typ[entity.AnalysisPhase](),
		toEntity:  func(m any) any { return mapper.AnalysisPhaseToEntity(m.(*models.AnalysisPhase)) },
		modelOnly: only(modelOnlyReason, "CreatedAt", "UpdatedAt"),
	},
	{
		name: "UserAnalysisProgress", model: typ[models.UserAnalysisProgress](), entity: typ[entity.UserAnalysisProgress](),
		toEntity:      func(m any) any { return mapper.UserAnalysisProgressToEntity(m.(*models.UserAnalysisProgress)) },
		fromEntity:    func(e any) any { return mapper.UserAnalysisProgressFromEntity(e.(*entity.UserAnalysisProgress)) },
		typeDiffers:   only("関連(Preload)。entity と model で型が別。ToEntity が AnalysisPhaseToEntity で変換する", "Phase"),
		notFromEntity: only("関連は書き戻さない。保存には外部キー(PhaseID)を使う", "Phase"),
	},
}

// filler は構造体の全ての公開項目を、項目ごとに別の、ゼロ値でない値で埋める。
// 値を項目ごとに変えるので、項目の取り違え(A を B に入れる)も検出できる。
type filler struct{ n int }

func (f *filler) fill(v reflect.Value, depth int) {
	if depth > 3 {
		return
	}
	f.n++
	switch v.Kind() {
	case reflect.String:
		v.SetString("s" + string(rune('A'+f.n%26)) + time.Duration(f.n).String())
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(f.n))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(f.n))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(f.n) + 0.5)
	case reflect.Ptr:
		p := reflect.New(v.Type().Elem())
		f.fill(p.Elem(), depth+1)
		v.Set(p)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		f.fill(s.Index(0), depth+1)
		v.Set(s)
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(f.n) * time.Hour)))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				f.fill(v.Field(i), depth+1)
			}
		}
	}
}

func fieldNames(t reflect.Type) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			out[t.Field(i).Name] = true
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// 片方にしかない項目が、許可リストと一致すること。
func TestMapper_FieldSetsMatchAllowlist(t *testing.T) {
	for _, c := range mapperCases {
		t.Run(c.name, func(t *testing.T) {
			mf, ef := fieldNames(c.model), fieldNames(c.entity)

			gotModelOnly := map[string]bool{}
			for n := range mf {
				if !ef[n] {
					gotModelOnly[n] = true
				}
			}
			for n := range gotModelOnly {
				if _, ok := c.modelOnly[n]; !ok {
					t.Errorf("model にだけ項目 %s がある。entity と mapper に足すか、許可リスト(modelOnly)に理由つきで載せること", n)
				}
			}
			for n := range c.modelOnly {
				if !gotModelOnly[n] {
					t.Errorf("modelOnly の %s は、model にだけある項目ではなくなった。リストから消すこと", n)
				}
			}
			for n := range ef {
				if !mf[n] {
					t.Errorf("entity にだけ項目 %s がある。model と mapper に足すこと(entity だけにある値は、保存されない)", n)
				}
			}
		})
	}
}

// ToEntity が、両方にある項目をそのまま渡すこと。
func TestMapper_ToEntityCopiesEverySharedField(t *testing.T) {
	for _, c := range mapperCases {
		t.Run(c.name, func(t *testing.T) {
			m := reflect.New(c.model)
			(&filler{}).fill(m.Elem(), 0)

			e := reflect.ValueOf(c.toEntity(m.Interface())).Elem()
			mf := fieldNames(c.model)
			for name := range fieldNames(c.entity) {
				if !mf[name] {
					continue
				}
				got, want := e.FieldByName(name), m.Elem().FieldByName(name)
				if got.Type() != want.Type() {
					if _, ok := c.typeDiffers[name]; !ok {
						t.Errorf("%s: 型が違う(entity %s / model %s)。typeDiffers に理由つきで載せること", name, got.Type(), want.Type())
					}
					continue
				}
				if !reflect.DeepEqual(got.Interface(), want.Interface()) {
					t.Errorf("ToEntity が %s を渡していない: entity=%v model=%v", name, got.Interface(), want.Interface())
				}
			}
		})
	}
}

// FromEntity が、両方にある項目をそのまま model に戻すこと。
func TestMapper_FromEntityCopiesEverySharedField(t *testing.T) {
	for _, c := range mapperCases {
		if c.fromEntity == nil {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			e := reflect.New(c.entity)
			(&filler{}).fill(e.Elem(), 0)

			m := reflect.ValueOf(c.fromEntity(e.Interface())).Elem()
			mf := fieldNames(c.model)
			for name := range fieldNames(c.entity) {
				if !mf[name] {
					continue
				}
				if _, ok := c.notFromEntity[name]; ok {
					continue
				}
				got, want := m.FieldByName(name), e.Elem().FieldByName(name)
				if got.Type() != want.Type() {
					if _, ok := c.typeDiffers[name]; !ok {
						t.Errorf("%s: 型が違う(model %s / entity %s)。typeDiffers に理由つきで載せること", name, got.Type(), want.Type())
					}
					continue
				}
				if !reflect.DeepEqual(got.Interface(), want.Interface()) {
					t.Errorf("FromEntity が %s を戻していない: model=%v entity=%v", name, got.Interface(), want.Interface())
				}
			}
		})
	}
}

// 許可リストの理由が空でないこと(「なんとなく」で載せない)。
func TestMapper_AllowlistReasonsAreWritten(t *testing.T) {
	for _, c := range mapperCases {
		for _, list := range []map[string]string{c.modelOnly, c.typeDiffers, c.notFromEntity} {
			for _, name := range sortedKeys(list) {
				if list[name] == "" {
					t.Errorf("%s.%s: 許可リストに理由が書かれていない", c.name, name)
				}
			}
		}
	}
}
