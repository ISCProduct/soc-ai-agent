package houjinbangou

import "strings"

// numberLength は法人番号の桁数（検査用数字1桁 + 基礎番号12桁）。
const numberLength = 13

// ValidateNumber は法人番号の検査用数字（先頭1桁）を検証する。
//
// 国税庁の仕様では、先頭1桁が検査用数字、残り12桁が基礎番号で、
//
//	検査用数字 = 9 - (Σ(n=1..12) Pn × Qn) mod 9
//	Pn: 基礎番号の下位n桁目の数字 / Qn: n が奇数なら 1、偶数なら 2
//
// 登記に実在するかまでは判定できないが、桁の整合しない番号はここで落ちる。
// 外部APIを叩かずに済むため、ハードコードされた番号の回帰テストに使える。
//
// 仕様: https://www.houjin-bangou.nta.go.jp/documents/checkdigit.pdf
func ValidateNumber(number string) bool {
	n := strings.TrimSpace(number)
	if len(n) != numberLength {
		return false
	}
	check := int(n[0] - '0')
	if check < 0 || check > 9 {
		return false
	}
	sum := 0
	for i := 1; i <= numberLength-1; i++ {
		// n[numberLength-i] が基礎番号の下位 i 桁目。
		d := int(n[numberLength-i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		q := 1
		if i%2 == 0 {
			q = 2
		}
		sum += d * q
	}
	return check == 9-(sum%9)
}
