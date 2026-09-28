package textsim

import (
	"strings"
	"unicode"
)

// minContentRunes は内容語照合に必要な最小文字数（内容語だけを残したあとの長さ）。
//
// 1文字では bigram が作れず、MatchScore の部分文字列判定だけが効いて 1.0 になる。
// 「私」「3」のような1文字の根拠が満点で通ってしまうため、2文字を下限にする。
const minContentRunes = 2

// ContentRunes は内容語らしい文字だけを残した文字列を返す（#1566）。
//
// Normalize（NFKC・小文字化・約物除去）を通したうえで、
// 漢字・カタカナ・数字・ラテン文字だけを残し、**ひらがなを落とす**。
// 日本語の助詞・助動詞・活用語尾はほぼひらがななので、形態素解析器を持ち込まずに
// 「内容語だけ」に近いものが取り出せる（"部員数を3倍にすることができました"
// -> "部員数3倍"）。
//
// 精度ではなく依存を増やさないことを優先した近似である。落ちるもの・残るもの:
//   - ひらがな表記の内容語（"おもてなし" など）も落ちる
//   - 長音符「ー」は Script=Common なので落ちる（"サークル" -> "サクル"）。
//     needle 側と haystack 側で同じように落ちるので照合には影響しない
//
// ponytail: 形態素解析器（kagome 等）を入れれば品詞で正確に切れるが、
// 辞書込みで数十MBの依存が増える。#1566 の目的（述語末尾の流用と相槌を弾く）は
// この近似で足りている（実測は docs/wiki/scoring.md §2-4）。
func ContentRunes(s string) string {
	var b strings.Builder
	for _, r := range Normalize(s) {
		switch {
		case unicode.Is(unicode.Han, r), unicode.Is(unicode.Katakana, r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r < unicode.MaxASCII && unicode.IsLetter(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NewContent は内容語だけを残した Bigrams を返す（照合先＝haystack 用）。
// needle 側は ContentBestMatch が内部で内容語化するので、そちらを使うこと。
func NewContent(s string) Bigrams {
	return New(ContentRunes(s))
}

// ContentBestMatch は内容語だけを突き合わせて needle が h に現れる度合いを返す（#1566）。
// h は NewContent で作ること（生の New を渡すと助詞ごと比較され意味のない値になる）。
//
// BestMatch との違いは、発話の述語末尾（「〜することができました」等）や
// 相槌（「はい」）を流用しただけの根拠を弾けること。
// これらは全文の文字bigramでは 0.27〜1.00 に乗るが、内容語だけにすると
// 前者は 0.00〜0.14、後者は内容語が残らず 0.00 になる（実測）。
//
// 内容語を流用した捏造（「実引用＋事実の継ぎ足し」）は依然として弾けない。
// 正当な要約と同じ帯（0.29〜0.75）に入るため、しきい値では分離できない。
func ContentBestMatch(needle string, h Bigrams) float64 {
	content := ContentRunes(needle)
	if len([]rune(content)) < minContentRunes {
		return 0
	}
	return BestMatch(content, h)
}
