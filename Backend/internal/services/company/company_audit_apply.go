package company

import (
	"fmt"
	"sort"

	"gorm.io/gorm"
)

// AuditChange は1社ぶんの変更と、戻すための元の値(backup の1行)。
type AuditChange struct {
	ID     uint           `json:"id"`
	Name   string         `json:"name"`
	Before map[string]any `json:"before"`
	After  map[string]any `json:"after"`
}

// auditColumns は点検が触ってよい companies の列(PlanFix が作るキー)。
// --restore は外部の JSON ファイルを読むため、列名はここに無ければ受け付けない。
var auditColumns = map[string]bool{
	"industry":             true,
	"work_style":           true,
	"website_url":          true,
	"founded_year":         true,
	"employee_count":       true,
	"employee_count_basis": true,
}

// ApplyAuditChanges は changes を tx 上で適用する。restore=true なら After → Before へ戻す。
//
// 更新は「現在値が想定どおりの行だけ」にする。読み込みから適用までの間に管理画面や再取得ジョブが
// 同じ列を更新していたら、その行は上書きせず全体を失敗にする(呼び出し側がロールバックする)。
// backup 経由の戻しも同じ条件で行い、適用後に手で直された値を巻き戻さない。
func ApplyAuditChanges(tx *gorm.DB, changes []AuditChange, restore bool) error {
	for _, c := range changes {
		from, to := c.Before, c.After
		if restore {
			from, to = c.After, c.Before
		}
		if err := checkAuditColumns(from, to); err != nil {
			return fmt.Errorf("id=%d (%s): %w", c.ID, c.Name, err)
		}
		q := tx.Table("companies").Where("id = ?", c.ID)
		// 列の順序を固定する(map の反復順は実行ごとに変わる)。
		for _, col := range sortedKeys(from) {
			switch v := from[col].(type) {
			case string:
				// companies は utf8mb4_unicode_ci なので `col = ?` では大文字小文字と
				// 末尾空白の違いを取りこぼす。URL のパスは大文字小文字で別物になり得るため、
				// 「適用後に手で直された値は巻き戻さない」をバイト比較で成立させる。
				q = q.Where("CAST(`"+col+"` AS BINARY) = CAST(? AS BINARY)", v)
			case nil:
				q = q.Where("`" + col + "` IS NULL")
			default:
				q = q.Where("`"+col+"` = ?", v)
			}
		}
		res := q.Updates(to)
		if res.Error != nil {
			return fmt.Errorf("id=%d: %w", c.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("id=%d (%s): 現在値が想定と違うため更新しませんでした(読み込み後に別の更新が入った可能性)", c.ID, c.Name)
		}
	}
	return nil
}

// checkAuditColumns は from / to の列名が点検対象だけであることを確かめる。
// 列名は SQL へ識別子として埋めるので、未知の名前はここで止める。
func checkAuditColumns(from, to map[string]any) error {
	if len(from) == 0 || len(to) == 0 {
		return fmt.Errorf("before / after が空です")
	}
	for _, m := range []map[string]any{from, to} {
		for col := range m {
			if !auditColumns[col] {
				return fmt.Errorf("点検対象でない列が含まれています: %q", col)
			}
		}
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
