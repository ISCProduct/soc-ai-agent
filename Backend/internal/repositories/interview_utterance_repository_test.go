package repositories

import (
	"strings"
	"testing"

	"Backend/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestInterviewUtteranceRepository_CreateIsIdempotent は #1476 のレビュー指摘
// 「再試行する発話保存を冪等にする」の SQL 側の回帰テスト。
//
// クライアントは保存失敗を再試行するが、「サーバーはDBへ書けたが応答だけ失われた」失敗が
// 混ざるため、素の INSERT のままだと同じ発言がもう一件入り、書き起こし・スコア・
// LoRA学習データへ重複して伝播する。一意制約に当たった再送を no-op にする SQL を固定する。
//
// INSERT IGNORE（clause.OnConflict{DoNothing:true} が MySQL で生成する形）になっていないことも見る。
// IGNORE は一意制約以外のエラーまで警告へ落とし、黙って壊れた行を作る。
func TestInterviewUtteranceRepository_CreateIsIdempotent(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(
		mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true},
	)
	if err != nil {
		t.Fatalf("DryRun DB の生成に失敗: %v", err)
	}

	// DryRun では発行直前の SQL がコールバックの Statement に載る
	var sql string
	if err := db.Callback().Create().After("gorm:create").Register("capture_sql", func(tx *gorm.DB) {
		sql = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("コールバック登録に失敗: %v", err)
	}

	clientID := "utt-1"
	repo := NewInterviewUtteranceRepository(db)
	if err := repo.Create(&models.InterviewUtterance{
		SessionID:         1,
		Role:              "user",
		Text:              "自己紹介をします",
		ClientUtteranceID: &clientID,
	}); err != nil {
		t.Fatalf("Create が失敗: %v", err)
	}
	if sql == "" {
		t.Fatal("SQL を取得できなかった")
	}
	tests := []struct {
		name    string
		want    string
		present bool
		msg     string
	}{
		{
			name:    "client_utterance_id を書き込む",
			want:    "`client_utterance_id`",
			present: true,
			msg:     "IDが書かれないと一意制約が効かず再試行で二重保存される",
		},
		{
			name:    "重複時は no-op にする",
			want:    "ON DUPLICATE KEY UPDATE",
			present: true,
			msg:     "重複でエラーになると、保存済みの再送が失敗扱いになりUIに誤った警告が出る",
		},
		{
			name:    "INSERT IGNORE にはしない",
			want:    "INSERT IGNORE",
			present: false,
			msg:     "IGNORE は一意制約以外のエラーも警告へ落とし、黙って壊れた行を作る",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Contains(sql, tt.want); got != tt.present {
				t.Fatalf("SQL に %q が含まれる=%v want %v: %s\nSQL: %s", tt.want, got, tt.present, tt.msg, sql)
			}
		})
	}
}
