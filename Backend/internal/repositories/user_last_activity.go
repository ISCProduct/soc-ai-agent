package repositories

import "time"

// FindLastActivityByUsers は生徒ごとの最終活動日時を返す（#1573）。
//
// 最終活動 = 次のうち最も新しい時刻:
//   - チャットで自分が送った発言（chat_messages.role = 'user'）
//   - 応募状況の更新（user_application_statuses.updated_at）
//   - スコアの更新（user_weight_scores.updated_at。面接・職務経歴書の反映も含む）
//
// AIの返答は生徒の操作ではないので数えない。戻り値に無い user_id は活動の記録なし。
func (r *UserRepository) FindLastActivityByUsers(userIDs []uint) (map[uint]time.Time, error) {
	result := map[uint]time.Time{}
	if len(userIDs) == 0 {
		return result, nil
	}

	const q = `
SELECT user_id, MAX(at) AS last_at FROM (
  SELECT user_id, MAX(created_at) AS at FROM chat_messages
   WHERE user_id IN (?) AND role = 'user' GROUP BY user_id
  UNION ALL
  SELECT user_id, MAX(updated_at) AS at FROM user_application_statuses
   WHERE user_id IN (?) GROUP BY user_id
  UNION ALL
  SELECT user_id, MAX(updated_at) AS at FROM user_weight_scores
   WHERE user_id IN (?) GROUP BY user_id
) activity
WHERE at IS NOT NULL
GROUP BY user_id`

	var rows []struct {
		UserID uint
		LastAt time.Time
	}
	if err := r.db.Raw(q, userIDs, userIDs, userIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, x := range rows {
		result[x.UserID] = x.LastAt
	}
	return result, nil
}
