package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/bsv-blockchain/certifier-server-example/internal/config"
	"github.com/bsv-blockchain/certifier-server-example/internal/constants"
	"github.com/bsv-blockchain/certifier-server-example/internal/example_setup"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	gosdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

var CertifierAddress = "http://localhost"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := setupConfig(logger)
	if err != nil {
		os.Exit(1)
	}

	aliceWallet, cleanup, _, aliceIdentityKey, err := setupWallet(cfg, logger)
	if err != nil {
		os.Exit(1)
	}
	defer cleanup()

	masterCertificate, err := createCertificate(context.Background(), aliceWallet, cfg, aliceIdentityKey, logger)
	if err != nil {
		logger.Error("Failed to create certificate", "error", err)
		os.Exit(1)
	}

	resp, err := sendCertificateToServer(masterCertificate, cfg, logger)
	if err != nil {
		os.Exit(1)
	}

	err = handleResponse(resp, logger)
	if err != nil {
		os.Exit(1)
	}
}

func setupConfig(logger *slog.Logger) (*config.Config, error) {
	cfg, err := config.LoadConfig("", logger)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid configuration", "error", err)
		return nil, err
	}

	return cfg, nil
}

func setupWallet(cfg *config.Config, logger *slog.Logger) (*wallet.Wallet, func(), *primitives.PrivateKey, *primitives.PublicKey, error) {
	alicePrivateKey, err := primitives.PrivateKeyFromHex(cfg.UserWallet.PrivateKey)
	if err != nil {
		logger.Error("Invalid user private key", "error", err)
		return nil, nil, nil, nil, err
	}

	aliceIdentityKey := alicePrivateKey.PubKey()
	if aliceIdentityKey.ToDERHex() != cfg.UserWallet.IdentityKey {
		logger.Error("User identity key mismatch")
		return nil, nil, nil, nil, fmt.Errorf("user identity key mismatch")
	}

	alice := &example_setup.Setup{
		Environment: example_setup.Environment{
			BSVNetwork: defs.BSVNetwork(cfg.Server.Network),
			ServerURL:  cfg.Storage.URL,
		},
		IdentityKey:      aliceIdentityKey,
		PrivateKey:       alicePrivateKey,
		ServerPrivateKey: cfg.Storage.PrivateKey,
	}

	aliceWallet, cleanup := alice.CreateWallet(context.Background(), alicePrivateKey)

	return aliceWallet, cleanup, alicePrivateKey, aliceIdentityKey, nil
}

func createCertificate(ctx context.Context, aliceWallet *wallet.Wallet, cfg *config.Config, aliceIdentityKey *primitives.PublicKey, logger *slog.Logger) (*certificates.MasterCertificate, error) {
	fields := map[gosdk.CertificateFieldNameUnder50Bytes]string{
		constants.FirstNameField: "John",
		constants.LastNameField:  "Doe",
		constants.CountryField:   "US",
		constants.EmailField:     "john.doe@example.com",
	}

	certifierIdentityKey, err := primitives.PublicKeyFromString(cfg.CertifierWallet.IdentityKey)
	if err != nil {
		logger.Error("Invalid certifier identity key", "error", err)
		return nil, err
	}

	certifierCounterparty := gosdk.Counterparty{
		Type:         gosdk.CounterpartyTypeOther,
		Counterparty: certifierIdentityKey,
	}

	createCertificateResults, err := certificates.CreateCertificateFields(
		ctx,
		aliceWallet,
		certifierCounterparty,
		fields,
		false,
		"",
	)
	if err != nil {
		return nil, err
	}

	certificateFields := createCertificateResults.CertificateFields
	certificateMasterKeyring := createCertificateResults.MasterKeyring

	certificate := certificates.NewCertificate(
		constants.SupportedCertType,
		"",
		*aliceIdentityKey,
		*certifierIdentityKey,
		nil,
		certificateFields,
		[]byte(""),
	)

	masterCertificate := &certificates.MasterCertificate{
		Certificate:   *certificate,
		MasterKeyring: certificateMasterKeyring,
	}

	return masterCertificate, nil
}

func sendCertificateToServer(masterCertificate *certificates.MasterCertificate, cfg *config.Config, logger *slog.Logger) (*http.Response, error) {
	bytesToSend, err := json.Marshal(masterCertificate)
	if err != nil {
		logger.Error("Failed to marshal master certificate", "error", err)
		return nil, err
	}

	if cfg.Server.Port == "" {
		logger.Error("Server port is not configured")
		return nil, fmt.Errorf("server port is not configured")
	}

	CertifierAddress = fmt.Sprintf("%s:%s", CertifierAddress, cfg.Server.Port)

	logger.Info("Sending certificate to server", "address", CertifierAddress)
	resp, err := http.Post(CertifierAddress, constants.ContentTypeJSON, bytes.NewReader(bytesToSend)) //nolint:gosec,noctx // example code, variable URL is intentional
	if err != nil {
		logger.Error("Failed to send request to certifier", "error", err)
		return nil, err
	}

	return resp, nil
}

func handleResponse(resp *http.Response, logger *slog.Logger) error {
	defer resp.Body.Close() //nolint:errcheck // body close error is not actionable in this context

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Certificate signing failed", "status", resp.Status, "error", err, "response", "<failed to read response body>")
			return fmt.Errorf("certificate signing failed with status: %s (failed to read response body: %w)", resp.Status, err)
		}
		logger.Error("Certificate signing failed", "status", resp.Status, "response", string(bodyBytes))
		return fmt.Errorf("certificate signing failed with status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", "error", err)
		return err
	}

	signedCertificate, err := certificates.CertificateFromBinary(bodyBytes)
	if err != nil {
		logger.Error("Failed to parse signed certificate", "error", err)
		return err
	}

	fmt.Println("=== Certificate Signing Successful ===")
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Certificate Type: %s\n", signedCertificate.Type)
	fmt.Printf("Subject: %s\n", signedCertificate.Subject.ToDERHex())
	fmt.Printf("Certifier: %s\n", signedCertificate.Certifier.ToDERHex())
	logger.Info("Certificate validation completed successfully")
	fmt.Println("=====================================")

	return nil
}
