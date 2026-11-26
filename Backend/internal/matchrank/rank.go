// Package matchrank は推薦の並びを一箇所に決める。
//
// 同点のとき、DB の取得順で順位が変わらないようにする（SOCAIAGENT-674）。
// 副キーは学生と企業の CRC32、その次が company_id、最後が行の id。
// 課金・プラン・広告費・掲載優先度は使わない。学生が変わるとハッシュも変わるので、
// 特定の企業を全学生で恒常的に上に固定しない。
package matchrank

import (
	"hash/crc32"
	"strconv"
)

// OrderSQL は user_company_matches の並び。GORM の Order にそのまま渡す。
// match_tie_key は生成列で、式は TieKey と同じ CRC32(CONCAT(user_id, ':', company_id))。
// 式を ORDER BY に書くと索引の順で読めず、filesort に戻る。
const OrderSQL = "match_score DESC, match_tie_key ASC, company_id ASC, id ASC"

// TieKey は MySQL の CRC32(CONCAT(user_id, ':', company_id)) と同じ値。
// hash/crc32 の IEEE は zlib と同じ多項式で、MySQL の CRC32 と一致する。
func TieKey(userID, companyID uint) uint32 {
	return crc32.ChecksumIEEE([]byte(strconv.FormatUint(uint64(userID), 10) + ":" + strconv.FormatUint(uint64(companyID), 10)))
}

// Less は a を b より前に置くとき true。スコアが高い方が先。
func Less(scoreA float64, userA, companyA, idA uint, scoreB float64, userB, companyB, idB uint) bool {
	if scoreA != scoreB {
		return scoreA > scoreB
	}
	keyA, keyB := TieKey(userA, companyA), TieKey(userB, companyB)
	if keyA != keyB {
		return keyA < keyB
	}
	if companyA != companyB {
		return companyA < companyB
	}
	return idA < idB
}
