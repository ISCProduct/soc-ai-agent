// update-registry-status は、企業の法人番号から国税庁の法人番号公表サイトを引き、
// 法人種別（companies.corporate_kind）と登記の閉鎖（registry_closed_on）、確認した日時（registry_checked_at）を保存する。
// 学生向け企業相関図から、国の機関・地方公共団体と、閉鎖された企業（よく知る企業を除く）を除くために使う。
//
// 既定は確認だけで、DB には書き込まない（国税庁への問い合わせは行う。読み取りのみ・無料）。
//
//		cd Backend
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/update-registry-status                                   # 確認のみ（公開済みだけ）
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/update-registry-status --include-drafts --limit 50       # 未公開・仮登録も含める
//		HOUJIN_BANGOU_APP_ID=... go run ./cmd/update-registry-status --apply --backup before.json      # 保存する
//		go run ./cmd/update-registry-status --restore before.json                                      # 書き込む前の値に戻す
//
//	  - 対象は、有効・ゲスト投稿でない・法人番号がある企業。既定は公開済み（published）だけ。
//	    国の機関は、ほとんどが未公開（gBizINFO の調達・補助金から作られた仮登録）なので、
//	    相関図から国の機関を除くには --include-drafts が要る。
//	  - 登録の商号が企業名と一致しない企業は、書き込まずに一覧へ出す（法人番号の入力誤りの可能性）。
//	  - --apply には --backup（書く前の3列の値。既存のファイルは上書きしない）が必須。
//	    --restore は、書き込んだ値のままの企業だけを戻す（その後に変わった値は触らない）。
//	  - 本番（APP_ENV=production）や localhost 以外の DB への書き込みは --allow-production が必要（接続より先に確認する）。
//	  - 法人番号公表サイトのアプリケーションID（HOUJIN_BANGOU_APP_ID）は、ログにも出さない。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"Backend/internal/models"
	"Backend/internal/services/company"
	"Backend/internal/services/houjinbangou"
)

const dateLayout = "2006-01-02"

// registryState は、企業の3列（corporate_kind / registry_closed_on / registry_checked_at）の値。
type registryState struct {
	Kind      string     `json:"corporate_kind"`
	ClosedOn  *string    `json:"registry_closed_on"` // YYYY-MM-DD。nil は、閉鎖なし or 未確認
	CheckedAt *time.Time `json:"registry_checked_at"`
}

func (s registryState) sameValue(o registryState) bool {
	return s.Kind == o.Kind && ptrEq(s.ClosedOn, o.ClosedOn)
}

func ptrEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// backupRow は、書き込んだ企業（復元に使う）。Before は書く前、After は書いた値。
// CorporateNumber は照会に使った法人番号。書き込み・復元はこの番号のままの企業だけに行う。
type backupRow struct {
	CompanyID       uint          `json:"company_id"`
	CorporateNumber string        `json:"corporate_number"`
	Before          registryState `json:"before"`
	After           registryState `json:"after"`
}

type listed struct {
	ID     uint
	Name   string
	Reason string
}

// finder は法人番号公表サイトの検索（houjinbangou.Client が満たす）。
type finder interface {
	FindByNumber(ctx context.Context, corporateNumber string) (*houjinbangou.Corporation, error)
}

// store は DB の読み書き。テストでは偽物に差し替える。
type store interface {
	Targets(includeDrafts bool, limit int) ([]models.Company, error)
	Apply(rows []backupRow) (int64, error)
	Restore(rows []backupRow) (int64, error)
}

type options struct {
	Apply         bool
	Backup        string
	Restore       string
	IncludeDrafts bool
	Limit         int
	AllowProd     bool
}

func main() {
	var o options
	flag.BoolVar(&o.Apply, "apply", false, "DB に書き込む。無ければ確認のみ")
	flag.StringVar(&o.Backup, "backup", "", "書く前の値の保存先（--apply で必須。既存のファイルは上書きしない）")
	flag.StringVar(&o.Restore, "restore", "", "バックアップの企業を、書く前の値に戻す")
	flag.BoolVar(&o.IncludeDrafts, "include-drafts", false, "未公開・仮登録の企業も対象にする（国の機関を除くには必要）")
	flag.IntVar(&o.Limit, "limit", 0, "対象の上限（0 は無制限）")
	flag.BoolVar(&o.AllowProd, "allow-production", false, "本番・localhost 以外への書き込みを許す")
	flag.Parse()

	loadEnv()
	finderClient := houjinbangou.NewClientFromEnv()
	if err := checkPreconditions(o, dbHost(), os.Getenv("APP_ENV"), finderClient.Enabled()); err != nil {
		log.Fatal(err)
	}
	db, err := connectDB()
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	fmt.Printf("接続先: host=%s db=%s\n", dbHost(), os.Getenv("DB_NAME"))

	if err := run(context.Background(), o, &gormStore{db: db}, finderClient, os.Stdout, time.Now); err != nil {
		log.Fatal(err)
	}
}

// checkPreconditions は、接続より先に確認すること。書き込みの許可、--backup、アプリケーションID。
func checkPreconditions(o options, host, appEnv string, appIDSet bool) error {
	if o.Apply || o.Restore != "" {
		if err := writeAllowed(o.AllowProd, host, appEnv); err != nil {
			return err
		}
	}
	if o.Restore != "" {
		return nil // 復元は国税庁を引かない
	}
	if o.Apply && o.Backup == "" {
		return errors.New("--apply には --backup が必要です（元に戻せるように）")
	}
	if !appIDSet {
		return errors.New("HOUJIN_BANGOU_APP_ID が未設定です（国税庁の法人番号システム Web-API のアプリケーションID）")
	}
	return nil
}

func run(ctx context.Context, o options, st store, f finder, out io.Writer, now func() time.Time) error {
	if o.Restore != "" {
		return runRestore(st, o.Restore, out)
	}

	targets, err := st.Targets(o.IncludeDrafts, o.Limit)
	if err != nil {
		return fmt.Errorf("対象の取得に失敗: %w", err)
	}
	scope := "公開済み"
	if o.IncludeDrafts {
		scope = "未公開・仮登録を含む"
	}
	pf(out, "対象（有効・法人番号あり・%s）: %d 社\n\n", scope, len(targets))

	var (
		rows                       []backupRow
		mismatches, missing, fails []listed
		agencies, localGovs        int
		closed, unchanged, changed int
	)
	for i := range targets {
		c := &targets[i]
		corp, err := f.FindByNumber(ctx, c.CorporateNumber)
		if err != nil {
			fails = append(fails, listed{c.ID, c.Name, err.Error()})
			continue
		}
		if corp == nil {
			missing = append(missing, listed{c.ID, c.Name, "法人番号が国税庁に見つからない"})
			continue
		}
		if houjinbangou.NormalizeCompanyName(corp.Name) != houjinbangou.NormalizeCompanyName(c.Name) {
			mismatches = append(mismatches, listed{c.ID, c.Name, "登録の商号が企業名と一致しない: " + corp.Name})
			continue
		}
		after, err := toState(corp, now())
		if err != nil {
			fails = append(fails, listed{c.ID, c.Name, err.Error()})
			continue
		}
		switch after.Kind {
		case company.CorporateKindNationalAgency:
			agencies++
		case company.CorporateKindLocalGovernment:
			localGovs++
		}
		if after.ClosedOn != nil {
			closed++
		}
		before := stateOf(c)
		if before.CheckedAt != nil && before.sameValue(after) {
			unchanged++ // 確認済みで値が同じ。書かない
			continue
		}
		if before.CheckedAt != nil {
			changed++
		}
		rows = append(rows, backupRow{CompanyID: c.ID, CorporateNumber: c.CorporateNumber, Before: before, After: after})
	}

	pf(out, "結果: 保存する %d 社 / 変更なし %d 社 / 商号の不一致 %d 社 / 国税庁に無い %d 社 / 通信などの失敗 %d 社（対象 %d 社）\n",
		len(rows), unchanged, len(mismatches), len(missing), len(fails), len(targets))
	pf(out, "内訳: 国の機関 %d 社 / 地方公共団体 %d 社 / 登記が閉鎖 %d 社（商号が一致した企業のうち）。確認済みで値が変わった企業 %d 社\n", agencies, localGovs, closed, changed)
	printList(out, "商号が一致しなかった企業（書き込みません。法人番号の入力誤りの可能性があります。人が確認してください）", mismatches)
	printList(out, "国税庁に見つからなかった企業", missing)
	printList(out, "通信などで失敗した企業（再実行してください）", fails)
	pl(out, "\n出典表示の注意: 国税庁の法人番号公表サイトのデータを画面などに出すときは、"+
		"「国税庁法人番号公表サイト（国税庁）」と表示すること。利用規約は人が確認してください。")
	if !o.IncludeDrafts {
		pl(out, "注意: 公開済みだけを対象にしています。国の機関は未公開の仮登録がほとんどなので、相関図から除くには --include-drafts を付けて実行してください。")
	}

	if !o.Apply {
		pl(out, "\n確認のみです（DB には書き込んでいません）。書き込むには --apply --backup <ファイル> を付けてください。")
		return nil
	}
	if len(rows) == 0 {
		pl(out, "\n書き込む企業がありません。")
		return nil
	}
	if err := writeBackup(o.Backup, rows); err != nil {
		return fmt.Errorf("バックアップに失敗: %w", err)
	}
	n, err := st.Apply(rows)
	if err != nil {
		return fmt.Errorf("書き込みに失敗（全件ロールバック済み）: %w", err)
	}
	pf(out, "\n書き込みました: %d 社（バックアップ: %s）\n元に戻す: go run ./cmd/update-registry-status --restore %s\n", n, o.Backup, o.Backup)
	return nil
}

// pf / pl は標準出力への表示。書き込みの失敗は処理に影響しないので無視する。
func pf(out io.Writer, format string, a ...any) { _, _ = fmt.Fprintf(out, format, a...) }
func pl(out io.Writer, a ...any)                { _, _ = fmt.Fprintln(out, a...) }

func printList(out io.Writer, title string, items []listed) {
	if len(items) == 0 {
		return
	}
	pf(out, "\n%s:\n", title)
	for _, it := range items {
		pf(out, "  %-6d %s — %s\n", it.ID, it.Name, it.Reason)
	}
}

// toState は、国税庁の結果を、保存する値にする。確認した日時は秒に丸める（DATETIME の精度に合わせる）。
func toState(corp *houjinbangou.Corporation, now time.Time) (registryState, error) {
	if corp.Kind == "" {
		return registryState{}, errors.New("国税庁の結果に法人種別がない")
	}
	checked := now.Truncate(time.Second)
	s := registryState{Kind: corp.Kind, CheckedAt: &checked}
	if corp.IsClosed() {
		d, err := time.Parse(dateLayout, corp.CloseDate)
		if err != nil {
			return registryState{}, fmt.Errorf("閉鎖日を解釈できない: %q", corp.CloseDate)
		}
		ds := d.Format(dateLayout)
		s.ClosedOn = &ds
	}
	return s, nil
}

func stateOf(c *models.Company) registryState {
	s := registryState{Kind: c.CorporateKind, CheckedAt: c.RegistryCheckedAt}
	if c.RegistryClosedOn != nil {
		ds := c.RegistryClosedOn.Format(dateLayout)
		s.ClosedOn = &ds
	}
	return s
}

func writeBackup(path string, rows []backupRow) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, //nolint:gosec // 利用者が --backup で指定した保存先
		os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err // 既存のファイルは上書きしない
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(b)
	return err
}

// runRestore は、バックアップの企業を書く前の値に戻す。ただし、書き込んだ値のままの企業だけ（その後に変わった値は戻さない）。
func runRestore(st store, path string, out io.Writer) error {
	b, err := os.ReadFile(path) //nolint:gosec // 利用者が --restore で指定したファイル
	if err != nil {
		return fmt.Errorf("バックアップを読めません: %w", err)
	}
	var rows []backupRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return fmt.Errorf("バックアップの形式が不正: %w", err)
	}
	n, err := st.Restore(rows)
	if err != nil {
		return fmt.Errorf("復元に失敗（全件ロールバック済み）: %w", err)
	}
	pf(out, "書く前の値に戻しました: %d 社（バックアップ %d 社のうち、書き込んだ値のままだった企業）\n", n, len(rows))
	return nil
}

// gormStore は store の MySQL 実装。
type gormStore struct{ db *gorm.DB }

func (g *gormStore) Targets(includeDrafts bool, limit int) ([]models.Company, error) {
	q := g.db.Where("is_active = ? AND is_guest_entry = ? AND corporate_number IS NOT NULL AND corporate_number <> ''", true, false)
	if !includeDrafts {
		q = q.Where("data_status = ?", "published")
	}
	q = q.Order("id")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var cs []models.Company
	return cs, q.Find(&cs).Error
}

func (g *gormStore) Apply(rows []backupRow) (int64, error) {
	return g.update(rows, func(r backupRow) (registryState, registryState) { return r.Before, r.After })
}

func (g *gormStore) Restore(rows []backupRow) (int64, error) {
	return g.update(rows, func(r backupRow) (registryState, registryState) { return r.After, r.Before })
}

// update は、法人番号が照会時のままで、現在の値が from のままの企業だけを to に更新する
// （その間に変わった企業は触らない）。全件を1トランザクションで行う。
func (g *gormStore) update(rows []backupRow, pick func(backupRow) (from, to registryState)) (int64, error) {
	var total int64
	err := g.db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			from, to := pick(r)
			res := tx.Model(&models.Company{}).
				Where("id = ? AND corporate_number = ? AND corporate_kind = ? AND registry_closed_on <=> ? AND registry_checked_at <=> ?",
					r.CompanyID, r.CorporateNumber, from.Kind, from.ClosedOn, from.CheckedAt).
				Updates(map[string]any{
					"corporate_kind":      to.Kind,
					"registry_closed_on":  to.ClosedOn,
					"registry_checked_at": to.CheckedAt,
				})
			if res.Error != nil {
				return res.Error
			}
			total += res.RowsAffected
		}
		return nil
	})
	return total, err
}

// writeAllowed は、書き込んでよいか。本番・localhost 以外の DB は、--allow-production が無ければ止める。
func writeAllowed(allowProd bool, host, appEnv string) error {
	if allowProd {
		return nil
	}
	local := host == "localhost"
	if ip := net.ParseIP(host); ip != nil {
		local = ip.IsLoopback()
	}
	if appEnv == "production" || !local {
		return errors.New("本番または localhost 以外の DB への書き込みには --allow-production が必要です（接続先: " + host + "）")
	}
	return nil
}

func dbHost() string {
	if h := os.Getenv("DB_HOST"); h != "" {
		return h
	}
	return "localhost"
}

func loadEnv() {
	if os.Getenv("APP_ENV") == "production" {
		return
	}
	for _, p := range []string{".env", "../.env", "../../.env"} {
		abs, _ := filepath.Abs(p)
		if _, err := os.Stat(abs); err == nil {
			_ = godotenv.Load(abs)
			return
		}
	}
}

// connectDB は DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME だけで接続する（認証用の秘密値は要らない）。
func connectDB() (*gorm.DB, error) {
	user, dbname := os.Getenv("DB_USER"), os.Getenv("DB_NAME")
	if user == "" || dbname == "" {
		return nil, errors.New("DB_USER / DB_NAME が設定されていません")
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	cfg := mysqldrv.NewConfig()
	cfg.User, cfg.Passwd = user, os.Getenv("DB_PASSWORD")
	cfg.Net, cfg.Addr, cfg.DBName = "tcp", net.JoinHostPort(dbHost(), port), dbname
	// 確認日時の保存・読込み・復元の比較を、実行する端末の TZ に依らず一致させるため JST に固定する。
	cfg.ParseTime, cfg.Loc = true, time.FixedZone("JST", 9*60*60)
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	return gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{})
}
