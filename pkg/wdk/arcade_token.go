package wdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// DeriveArcadeCallbackToken derives a stable, instance-identifying callback token
// from the wallet identity public key (DER hex). Deterministic so the SSE stream
// scope and Last-Event-ID replay survive restarts.
func DeriveArcadeCallbackToken(identityKeyHex string) string {
	mac := hmac.New(sha256.New, []byte(identityKeyHex))
	mac.Write([]byte("go-wallet-toolbox/arcade-sse/v1"))
	return hex.EncodeToString(mac.Sum(nil))
}
