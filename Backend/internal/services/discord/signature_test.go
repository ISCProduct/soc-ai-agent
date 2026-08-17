package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	publicKeyHex := hex.EncodeToString(pub)
	timestamp := "1234567890"
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifySignature(tt.publicKeyHex, tt.signatureHex, tt.timestamp, tt.body); got != tt.want {
				t.Errorf("VerifySignature() = %v, want %v", got, tt.want)
			}
		})
	}
}
