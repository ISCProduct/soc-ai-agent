package matchrank

import (
	"hash/crc32"
	"strconv"
	"strings"
	"testing"
)

func TestTieKeyMatchesZlibCRC32(t *testing.T) {
	// Python zlib.crc32 と一致する固定値。MySQL の CRC32 も同じ多項式。
	tests := []struct {
		user, company uint
		want          uint32
	}{
		{1, 2, 932632908},
		{1, 4, 3740630137},
		{7, 2, 857449982},
		{7, 4, 3665297611},
	}
	for _, tt := range tests {
		got := TieKey(tt.user, tt.company)
		if got != tt.want {
			t.Errorf("TieKey(%d,%d)=%d want %d", tt.user, tt.company, got, tt.want)
		}
		raw := crc32.ChecksumIEEE([]byte(strconv.FormatUint(uint64(tt.user), 10) + ":" + strconv.FormatUint(uint64(tt.company), 10)))
		if got != raw {
			t.Errorf("TieKey が IEEE CRC32 と違う: %d vs %d", got, raw)
		}
	}
}

func TestOrderSQLUsesNeutralTieBreak(t *testing.T) {
	sql := OrderSQL
	for _, want := range []string{
		"match_score DESC",
		"CRC32(CONCAT(user_id, ':', company_id)) ASC",
		"company_id ASC",
		"id ASC",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("並びSQLに %q が無い: %s", want, sql)
		}
	}
	lower := strings.ToLower(sql)
	for _, banned := range []string{"plan", "paid", "priority", "advert", "billing", "boost"} {
		if strings.Contains(lower, banned) {
			t.Errorf("副キーに %q が入っている: %s", banned, sql)
		}
	}
}

func TestLessIsTotalAndIgnoresInput(t *testing.T) {
	// 同点なら CRC の小さい企業 2 が、入力順に関係なく企業 4 より前。
	if !Less(50, 1, 2, 9, 50, 1, 4, 1) {
		t.Fatal("企業2が企業4より前になっていない")
	}
	if Less(50, 1, 4, 1, 50, 1, 2, 9) {
		t.Fatal("企業4が企業2より前になっている")
	}
	if !Less(80, 1, 4, 1, 50, 1, 2, 9) {
		t.Fatal("高スコアが負けている")
	}
	if !Less(50, 1, 3, 1, 50, 1, 3, 2) {
		t.Fatal("同じ企業の行で id の小さい方が前になっていない")
	}
}
