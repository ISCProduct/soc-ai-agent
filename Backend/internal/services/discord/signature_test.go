package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestVerifySignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	publicKeyHex := hex.EncodeToString(pub)
	// 署名対象のタイムスタンプは鮮度検証の対象でもあるため、固定値ではなく
	// 「テスト実行時の現在時刻」で署名する。
	now := time.Unix(1757300000, 0)
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = time.Now })
	timestamp := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"type":1}`)
	message := append([]byte(timestamp), body...)
	signature := ed25519.Sign(priv, message)
	signatureHex := hex.EncodeToString(signature)

	tests := []struct {
		name         string
		publicKeyHex string
		signatureHex string
		timestamp    string
		body         []byte
		want         bool
	}{
		{"valid signature", publicKeyHex, signatureHex, timestamp, body, true},
		{"tampered body", publicKeyHex, signatureHex, timestamp, []byte(`{"type":2}`), false},
		{"wrong timestamp", publicKeyHex, signatureHex, "0000000000", body, false},
		{"invalid public key hex", "not-hex", signatureHex, timestamp, body, false},
		{"invalid signature hex", publicKeyHex, "not-hex", timestamp, body, false},
		{"empty signature", publicKeyHex, "", timestamp, body, false},
	}
	// 署名済みリクエストのリプレイを防ぐ。/prod state:off を後から何度でも
	// 再送できると、本番を止められてしまう。
	for _, skew := range []time.Duration{-10 * time.Minute, 10 * time.Minute} {
		ts := strconv.FormatInt(now.Add(skew).Unix(), 10)
		sig := hex.EncodeToString(ed25519.Sign(priv, append([]byte(ts), body...)))
		tests = append(tests, struct {
			name         string
			publicKeyHex string
			signatureHex string
			timestamp    string
			body         []byte
			want         bool
		}{"古い/先の署名は拒否する (" + skew.String() + ")", publicKeyHex, sig, ts, body, false})
	}
	// 許容範囲内(4分)なら通す。時計ズレで正常な操作を弾かないこと。
	{
		ts := strconv.FormatInt(now.Add(-4*time.Minute).Unix(), 10)
		sig := hex.EncodeToString(ed25519.Sign(priv, append([]byte(ts), body...)))
		tests = append(tests, struct {
			name         string
			publicKeyHex string
			signatureHex string
			timestamp    string
			body         []byte
			want         bool
		}{"許容範囲内の時計ズレは通す", publicKeyHex, sig, ts, body, true})
	}
	tests = append(tests, struct {
		name         string
		publicKeyHex string
		signatureHex string
		timestamp    string
		body         []byte
		want         bool
	}{"数値でないタイムスタンプは拒否する", publicKeyHex, signatureHex, "not-a-number", body, false})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifySignature(tt.publicKeyHex, tt.signatureHex, tt.timestamp, tt.body); got != tt.want {
				t.Errorf("VerifySignature() = %v, want %v", got, tt.want)
			}
		})
	}
}
