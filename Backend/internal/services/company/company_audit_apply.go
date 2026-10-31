package company

import (
	"fmt"

	"gorm.io/gorm"
)

// AuditChange は1社ぶんの変更と、戻すための元の値(backup の1行)。
type AuditChange struct {
	ID     uint           `json:"id"`
	Name   string         `json:"name"`
	Before map[string]any `json:"before"`
	After  map[string]any `json:"after"`
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
		res := tx.Table("companies").Where("id = ?", c.ID).Where(from).Updates(to)
		if res.Error != nil {
			return fmt.Errorf("id=%d: %w", c.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("id=%d (%s): 現在値が想定と違うため更新しませんでした(読み込み後に別の更新が入った可能性)", c.ID, c.Name)
		}
	}
	return nil
}
