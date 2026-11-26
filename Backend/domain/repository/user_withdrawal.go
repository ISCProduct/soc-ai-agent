package repository

import (
	"Backend/internal/models"
	"context"
	"errors"
	"time"
)

// ErrAlreadyWithdrawn は、すでに退会済みのユーザーを退会させようとしたことを表す。
var ErrAlreadyWithdrawn = errors.New("account already withdrawn")

// ErrNotStaleGuest は、掃除の一覧を取ったあとで、ゲストでなくなった(本登録した)か、使われたことを表す(SOCAIAGENT-689)。
var ErrNotStaleGuest = errors.New("user is no longer a stale guest")

// WithdrawalRecordBuilder は退会の記録(withdrawn_users の1行)を作る。
// Withdraw のトランザクションの中で、読んだユーザーと、そのユーザーが持つオブジェクトの
// キー(動画・履歴書の保存先。S3 の URI のままのこともある)を受け取って呼ばれる。
// メールのハッシュ化やキーの正規化は、DB の外の手順なので、呼び出し側が持つ。
type WithdrawalRecordBuilder func(user *models.User, rawObjectKeys []string) models.WithdrawnUser

// UserWithdrawalRepository は退会(論理削除)と、猶予後の物理パージの DB 操作を担う。
type UserWithdrawalRepository interface {
	// Withdraw は論理退会を1トランザクションで行う。
	//   - 退会済みなら ErrAlreadyWithdrawn(何も書かない)
	//   - build が返した退会の記録を保存する
	//   - 元のメールアドレスの仮登録を消す(メールをスクランブルする前に)
	//   - メール・パスワード・OAuth・トークンを消して、論理退会の印(withdrawn_at)を付ける
	//   - 全端末のリフレッシュトークンを失効させる
	// どれか1つでも失敗したら、全体を巻き戻す。
	Withdraw(userID uint, now time.Time, build WithdrawalRecordBuilder) error

	// WithdrawStaleGuest は Withdraw と同じだが、同じトランザクションの中で、まだ掃除の対象か
	// (ゲスト・管理者でない・作成が cutoff より前・cutoff 以降のリフレッシュトークンが無い)を確かめる。
	// 外れていれば ErrNotStaleGuest を返し、何も書かない(SOCAIAGENT-689)。
	WithdrawStaleGuest(ctx context.Context, userID uint, cutoff, now time.Time, build WithdrawalRecordBuilder) error

	// HardDeleteUser はユーザーとその関連データを、1トランザクションで物理削除する。
	// collectiveAnonymousID は、集合知ログ(匿名化済み)を消すためのハッシュ。
	// ユーザーが既に存在しなければ、何もせず成功とする。
	HardDeleteUser(userID uint, collectiveAnonymousID string) error

	// ListDueWithdrawals は、保持期間(purge_after)を過ぎて、まだパージされていない退会の記録を返す。
	ListDueWithdrawals(now time.Time) ([]models.WithdrawnUser, error)
	// ListPendingWithdrawals は、まだパージされていない退会の記録を全て返す。
	ListPendingWithdrawals() ([]models.WithdrawnUser, error)
	// MarkPurged は退会の記録にパージ済みの日時を付ける。
	MarkPurged(withdrawnUserID uint, at time.Time) error

	// FindWithdrawalState は users の id と withdrawn_at だけを読む(認証のたびに呼ばれる)。
	// 行が無ければ found=false(物理削除済み)。
	FindWithdrawalState(userID uint) (user *models.User, found bool, err error)

	// ListStaleGuestIDs は、作成が cutoff より前で、cutoff 以降にリフレッシュトークンが発行されていない
	// 未退会のゲスト(管理者を除く)の ID を、ID 順に最大 limit 件返す(SOCAIAGENT-689)。
	ListStaleGuestIDs(ctx context.Context, cutoff time.Time, limit int) ([]uint, error)
}
