package discord

import (
	"crypto/ed25519"
	"encoding/hex"
)

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
	message := append([]byte(timestamp), body...)
	return ed25519.Verify(publicKey, message, signature)
}
