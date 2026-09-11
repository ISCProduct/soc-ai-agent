package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"strconv"
	"time"
)

// signatureMaxSkew は署名の許容時刻ズレ。
// これが無いと、一度観測された署名済みリクエストを後から何度でも再送できる。
// /prod state:off のような本番を止める操作でこれを許すと影響が大きい。
const signatureMaxSkew = 5 * time.Minute

// timeNow はテストから差し替えるための時刻取得。
var timeNow = time.Now

// VerifySignature は Discord Interactions Endpoint の必須要件である
// Ed25519 署名検証を行う。
// https://discord.com/developers/docs/interactions/receiving-and-responding#security-and-authorization
func VerifySignature(publicKeyHex, signatureHex, timestamp string, body []byte) bool {
	publicKey, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	signature, err := hex.DecodeString(signatureHex)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return false
	}
	if !timestampFresh(timestamp) {
		return false
	}
	message := append([]byte(timestamp), body...)
	return ed25519.Verify(publicKey, message, signature)
}

// timestampFresh はDiscordが署名対象に含めるUNIX秒が現在時刻から
// signatureMaxSkew 以内かを返す。過去・未来の両方向を見る。
func timestampFresh(timestamp string) bool {
	sec, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	diff := timeNow().Sub(time.Unix(sec, 0))
	if diff < 0 {
		diff = -diff
	}
	return diff <= signatureMaxSkew
}
