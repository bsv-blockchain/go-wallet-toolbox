// A single-merchant POS gateway backed by the existing Go Wallet Toolbox cloud
// storage adapter. Bind locally and terminate HTTPS at the deployment edge.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/pos"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	keyFile, storageURL, publicURL, name := os.Getenv("POS_MERCHANT_KEY_FILE"), os.Getenv("POS_STORAGE_URL"), os.Getenv("POS_PUBLIC_URL"), os.Getenv("POS_MERCHANT_NAME")
	if keyFile == "" || !strings.HasPrefix(storageURL, "https://") || publicURL == "" || name == "" {
		return errors.New("configure POS_MERCHANT_KEY_FILE, POS_STORAGE_URL (HTTPS), POS_PUBLIC_URL (HTTPS), and POS_MERCHANT_NAME")
	}
	// Mount this file from the deployment secret store, never from a cashier.
	raw, err := os.ReadFile(keyFile) // #nosec G304,G703 -- Operator-configured mounted secret path; never derived from an RPC request.
	if err != nil {
		return err
	}
	private, err := ec.PrivateKeyFromHex(strings.TrimSpace(string(raw)))
	if err != nil {
		return errors.New("invalid merchant key file")
	}
	chain := defs.NetworkTestnet
	chainName := "test"
	if os.Getenv("POS_CHAIN") == "main" {
		chain = defs.NetworkMainnet
		chainName = "main"
	} else if v := os.Getenv("POS_CHAIN"); v != "" && v != "test" {
		return errors.New("POS_CHAIN must be main or test")
	}
	merchant, err := wallet.NewWithStorageFactory(chain, private, func(w sdk.Interface) (wdk.WalletStorageProvider, func(), error) {
		return storage.NewClient(storageURL, w)
	})
	if err != nil {
		return err
	}
	defer merchant.Close()
	stateFile := os.Getenv("POS_STATE_FILE")
	if stateFile == "" {
		stateFile = "./pos-state.json"
	}
	service, err := pos.New(pos.Config{Wallet: merchant, Merchant: private.PubKey().ToDERHex(), Name: name, URL: publicURL, Chain: chainName, Store: pos.FileStore{Path: stateFile}, Accept: pos.MerchantAcceptor(merchant)})
	if err != nil {
		return err
	}
	address := os.Getenv("POS_LISTEN_ADDR")
	if address == "" {
		address = "127.0.0.1:8787"
	}
	server := &http.Server{Addr: address, Handler: service.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	fmt.Println("Point of sale gateway listening behind HTTPS proxy")
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
