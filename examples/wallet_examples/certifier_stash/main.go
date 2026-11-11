package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	walletcerts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/certificates"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/certifier"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

// stashDSN is where issued certificates are stashed. Use "" for an in-memory database.
const stashDSN = "certifier-stash-demo.sqlite"

// This example issues a certificate with the certifier service and reads it back from the stash.
// It runs fully in-process: no storage server, no HTTP and no funds are needed, because the certifier
// wallet below fakes CreateAction (the revocation output) instead of building a real transaction.
func main() {
	ctx := context.Background()

	certifierWallet := newOfflineCertifierWallet()
	subjectKey := must(ec.NewPrivateKey())
	subjectWallet := must(sdk.NewCompletedProtoWallet(subjectKey))
	certifierKey := must(certifierWallet.GetPublicKey(ctx, sdk.GetPublicKeyArgs{IdentityKey: true}, "")).PublicKey

	stash := must(certifier.NewSQLiteStash(stashDSN))
	defer func() { _ = stash.Close() }()

	service := certifier.NewCertificateService(certifierWallet, &certifier.ServerConfig{
		Logger:     logging.New().WithLevel("info").WithHandler("text", os.Stdout).Logger(),
		Originator: "certifier-stash-demo",
		Randomizer: randomizer.New(),
		Stash:      stash,
	})

	// 1. The subject encrypts its fields for the certifier, as the wallet does in acquireCertificate.
	encrypted := must(certificates.CreateCertificateFields(ctx, subjectWallet,
		sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: certifierKey},
		map[sdk.CertificateFieldNameUnder50Bytes]string{
			"firstName": "Alice",
			"email":     "alice@example.com",
		}, false, ""))

	req := &walletcerts.ProtocolIssuanceRequest{
		Type:          base64.StdEncoding.EncodeToString([]byte("demo certificate type")),
		Nonce:         base64.StdEncoding.EncodeToString(randomBytes(32)),
		Fields:        toStringMap(encrypted.CertificateFields),
		MasterKeyring: toStringMap(encrypted.MasterKeyring),
	}

	// 2. The certifier signs it (this is what POST /signCertificate calls) and stashes it.
	resp := must(service.SignCertificate(ctx, req, subjectKey.PubKey()))
	fmt.Println("Issued certificate")
	fmt.Println("  serial number:      ", resp.Certificate.SerialNumber)
	fmt.Println("  subject:            ", resp.Certificate.Subject)
	fmt.Println("  revocation outpoint:", resp.Certificate.RevocationOutpoint)

	// 3. Read it back from the stash.
	stashed := must(stash.FindBySerialNumber(ctx, resp.Certificate.SerialNumber))
	fmt.Printf("\nStashed certificate (id %d, created %s)\n", stashed.ID, stashed.CreatedAt.Format("15:04:05"))
	fmt.Println("  revocation tx bytes:", len(stashed.RevocationTx))
	for _, f := range stashed.Fields {
		fmt.Printf("  field %-10s value %.24s...  masterKey %.24s...\n", f.Name, f.Value, f.MasterKey)
	}

	bySubject := must(stash.FindBySubject(ctx, resp.Certificate.Subject))
	fmt.Printf("\nCertificates stashed for this subject: %d\n", len(bySubject))
	if stashDSN != "" {
		fmt.Println("Stash database:", stashDSN)
	}
}

// offlineCertifierWallet is a key-only wallet whose CreateAction returns a random txid
// instead of creating and broadcasting the revocation transaction.
type offlineCertifierWallet struct {
	*sdk.CompletedProtoWallet
}

func newOfflineCertifierWallet() *offlineCertifierWallet {
	return &offlineCertifierWallet{CompletedProtoWallet: must(sdk.NewCompletedProtoWallet(must(ec.NewPrivateKey())))}
}

func (w *offlineCertifierWallet) CreateAction(_ context.Context, _ sdk.CreateActionArgs, _ string) (*sdk.CreateActionResult, error) {
	return &sdk.CreateActionResult{
		Txid: chainhash.DoubleHashH(randomBytes(32)),
		Tx:   []byte("fake revocation tx"),
	}, nil
}

func toStringMap(in map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[string(k)] = string(v)
	}
	return out
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_ = must(rand.Read(b))
	return b
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
