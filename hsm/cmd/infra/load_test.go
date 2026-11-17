//go:build pkcs11

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/hsm"
	"github.com/bsv-blockchain/go-wallet-toolbox/hsm/pkcs11"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// BenchmarkGrimselAuth measures the server wallet crypto in an authenticated
// request/response: nonce HMAC verification, request signature verification with
// a fresh key ID (cache miss), and response signing. It excludes HTTP, storage,
// and the initial handshake; it is a crypto capacity ceiling, not an HTTP SLA.
func BenchmarkGrimselAuth(b *testing.B) {
	lib, pin, label := os.Getenv("HSM_PKCS11_LIB"), os.Getenv("HSM_PIN_FILE"), os.Getenv("HSM_KEY_LABEL")
	if lib == "" || pin == "" || label == "" {
		b.Skip("set HSM_PKCS11_LIB, HSM_PIN_FILE, HSM_KEY_LABEL")
	}
	poolSize := 4
	if raw := os.Getenv("HSM_POOL_SIZE"); raw != "" {
		n, err := strconv.Atoi(raw)
		require.NoError(b, err)
		poolSize = n
	}
	for _, workers := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("pool%d/clients%d", poolSize, workers), func(b *testing.B) {
			backend, err := pkcs11.Open(b.Context(), pkcs11.Config{LibraryPath: lib, PINFile: pin, KeyLabel: label, Slot: -1, SessionPoolSize: poolSize})
			require.NoError(b, err)
			d, err := hsm.New(b.Context(), backend)
			require.NoError(b, err)
			defer func() { require.NoError(b, d.Close()) }()
			// No transaction/storage method is called by this benchmark.
			w, err := wallet.NewWithHSM(defs.NetworkMainnet, d, &unusedStorage{})
			require.NoError(b, err)
			peerKey, err := ec.NewPrivateKey()
			require.NoError(b, err)
			peer, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypePrivateKey, PrivateKey: peerKey})
			require.NoError(b, err)
			peerKD := sdk.NewKeyDeriver(peerKey)
			owner := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: d.IdentityKey()}
			other := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: peerKey.PubKey()}
			nonce := []byte("fixed nonce data")
			nonceArgs := sdk.EncryptionArgs{ProtocolID: sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryApp, Protocol: "server hmac"}, KeyID: string(nonce), Counterparty: sdk.Counterparty{Type: sdk.CounterpartyTypeSelf}}
			mac, err := w.CreateHMAC(b.Context(), sdk.CreateHMACArgs{EncryptionArgs: nonceArgs, Data: nonce}, "")
			require.NoError(b, err)
			protocol := sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty, Protocol: "auth message signature"}
			payload := []byte("authenticated request benchmark")
			signatures := make([]*ec.Signature, b.N)
			for i := range signatures {
				sig, err := peer.CreateSignature(b.Context(), sdk.CreateSignatureArgs{EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: fmt.Sprintf("request %d", i), Counterparty: owner}, Data: payload}, "")
				require.NoError(b, err)
				signatures[i] = sig.Signature
			}
			durations := make([]time.Duration, b.N)
			var next atomic.Int64
			var wg sync.WaitGroup
			b.ResetTimer()
			started := time.Now()
			for range workers {
				wg.Go(func() {
					for {
						i := int(next.Add(1) - 1)
						if i >= b.N {
							return
						}
						begin := time.Now()
						ctx, cancel := context.WithTimeout(b.Context(), time.Minute)
						err := authRoundTrip(ctx, w, peerKD, nonceArgs, nonce, mac.HMAC, protocol, other, owner, payload, signatures[i], i)
						cancel()
						if err != nil {
							b.Errorf("request %d: %v", i, err)
							return
						}
						durations[i] = time.Since(begin)
					}
				})
			}
			wg.Wait()
			elapsed := time.Since(started)
			b.StopTimer()
			if b.Failed() {
				return
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			b.ReportMetric(float64(b.N)/elapsed.Seconds(), "requests/s")
			b.ReportMetric(float64(durations[(b.N-1)/2])/float64(time.Millisecond), "p50-ms")
			b.ReportMetric(float64(durations[(b.N-1)*95/100])/float64(time.Millisecond), "p95-ms")
		})
	}
}

type unusedStorage struct{ wdk.WalletStorageProvider }

func authRoundTrip(ctx context.Context, w *wallet.Wallet, peer *sdk.KeyDeriver, nonceArgs sdk.EncryptionArgs, nonce []byte, mac [32]byte, protocol sdk.Protocol, other, owner sdk.Counterparty, payload []byte, sig *ec.Signature, i int) error {
	validMAC, err := w.VerifyHMAC(ctx, sdk.VerifyHMACArgs{EncryptionArgs: nonceArgs, Data: nonce, HMAC: mac}, "")
	if err != nil {
		return err
	}
	if !validMAC.Valid {
		return fmt.Errorf("nonce HMAC rejected")
	}
	validSig, err := w.VerifySignature(ctx, sdk.VerifySignatureArgs{EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: fmt.Sprintf("request %d", i), Counterparty: other}, Data: payload, Signature: sig}, "")
	if err != nil {
		return err
	}
	if !validSig.Valid {
		return fmt.Errorf("request signature rejected")
	}
	responseID := fmt.Sprintf("response %d", i)
	response, err := w.CreateSignature(ctx, sdk.CreateSignatureArgs{EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: responseID, Counterparty: other}, Data: payload}, "")
	if err != nil {
		return err
	}
	pub, err := peer.DerivePublicKey(protocol, responseID, owner, false)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	if !response.Signature.Verify(digest[:], pub) {
		return fmt.Errorf("response signature rejected")
	}
	return nil
}
