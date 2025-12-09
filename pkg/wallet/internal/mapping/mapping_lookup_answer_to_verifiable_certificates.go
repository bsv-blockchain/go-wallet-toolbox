package mapping

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
)

func MapLookupAnswerToVerifiableCertificates(ctx context.Context, logger *slog.Logger, lookupAns *lookup.LookupAnswer) []certificates.VerifiableCertificate {
	if lookupAns.Type != lookup.AnswerTypeOutputList {
		return nil
	}

	parsedCerts := make([]certificates.VerifiableCertificate, 0)

	// On any error we do nothing and continue with other outputs as in TS
	for _, output := range lookupAns.Outputs {
		tx, err := transaction.NewTransactionFromBEEF(output.Beef)
		if err != nil {
			logger.Error("failed to create tx from output beef", slog.String("beef", string(output.Beef)), logging.Error(err))
			continue
		}

		// Decode the Identity token fields from the Bitcoin outputScript
		decodedOutput := pushdrop.Decode(tx.Outputs[output.OutputIndex].LockingScript)

		// Parse out the certificate and relevant data
		var verifiableCertificate certificates.VerifiableCertificate
		err = json.Unmarshal(decodedOutput.Fields[0], &verifiableCertificate)
		if err != nil {
			logger.Error("failed to unmarshal decodedOutput field into a certificate", logging.Error(err))
			continue
		}

		anyoneProtoWallet, err := wallet.NewCompletedProtoWallet(nil)
		if err != nil {
			logger.Error("failed to create anyone's proto wallet", logging.Error(err))
			continue
		}

		decryptedFields, err := verifiableCertificate.DecryptFields(ctx,
			anyoneProtoWallet,
			false,
			"")
		if err != nil {
			logger.Error("failed to decrypt verifiableCertificate fields", logging.Error(err))
			continue
		}

		// Verify the certificate signature is correct
		err = verifiableCertificate.Verify(ctx)
		if err != nil {
			logger.Error("failed to verify certificate's signature", logging.Error(err))
			continue
		}

		verifiableCertificate.DecryptedFields = decryptedFields

		parsedCerts = append(parsedCerts, verifiableCertificate)
	}

	return parsedCerts
}
