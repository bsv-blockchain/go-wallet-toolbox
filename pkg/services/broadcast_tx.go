package services

import (
	"encoding/hex"
	"sync"
)

// broadcastTx is one transaction on its way to the broadcasters, in every form they take: EF
// bytes (Arcade, PostEFBatch implementations), raw bytes (PostTX) and EF hex (ARC, PostEF
// implementations). The hex is encoded at most once, and only when a broadcaster asks for it.
type broadcastTx struct {
	ef    []byte
	rawTx []byte

	hexOnce sync.Once
	hex     string
}

func newBroadcastTx(ef, rawTx []byte) *broadcastTx {
	return &broadcastTx{ef: ef, rawTx: rawTx}
}

// efHex returns the hex of the EF. It is safe for concurrent use.
func (t *broadcastTx) efHex() string {
	t.hexOnce.Do(func() {
		if t.hex == "" {
			t.hex = hex.EncodeToString(t.ef)
		}
	})
	return t.hex
}
