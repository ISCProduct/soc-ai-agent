package models

import (
	"Backend/internal/services/houjinbangou"
	"testing"
)

// TestSeedCompanyRelationsCorporateNumbers はシードにハードコードされた法人番号が
// 検査用数字の整合する値であることを検証する。
//
// かつてこのシードには登記に存在しない法人番号が 8 件含まれており、親会社側の
// gBizINFO 同期が永久に 404 で失敗していた。外部APIを叩かずに再発を検出する。
func TestSeedCompanyRelationsCorporateNumbers(t *testing.T) {
	type entry struct {
		label  string
		number string
	}

	var entries []entry
	for _, spec := range factCapitalRelations {
		entries = append(entries,
			entry{label: spec.parentName, number: spec.parentCorpNum},
			entry{label: spec.childName, number: spec.childCorpNum},
		)
	}
	for _, spec := range factBusinessRelations {
		entries = append(entries,
			entry{label: spec.fromName, number: spec.fromCorpNum},
			entry{label: spec.toName, number: spec.toCorpNum},
		)
	}

	if len(entries) == 0 {
		t.Fatal("シードに法人番号が 1 件も無い")
	}

	for _, e := range entries {
		t.Run(e.label+"/"+e.number, func(t *testing.T) {
			if !houjinbangou.ValidateNumber(e.number) {
				t.Errorf("法人番号 %q (%s) の検査用数字が不正。国税庁APIで正しい番号を確認すること", e.number, e.label)
			}
		})
	}
}
