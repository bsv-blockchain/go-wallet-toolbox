package mapping

import (
	"log/slog"
	"slices"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_settings_manager"
	"github.com/go-softwarelab/common/pkg/to"
)

// identityGroup is a helper type used for grouping certificates by subject
type identityGroup struct {
	TotalTrust int
	Members    []wallet.IdentityCertificate
}

// MapVerifiableCertificatesWithTrust transforms an array of VerifiableCertificate
// instances according to the trust settings.
// Only certificates whose grouped total trust meets the threshold are returned,
// and each certificate is augmented with a certifierInfo property.
func MapVerifiableCertificatesWithTrust(logger *slog.Logger, trustSettings *wallet_settings_manager.TrustSettings, certificates []certificates.VerifiableCertificate) (*wallet.DiscoverCertificatesResult, error) {
	if logger == nil {
		logger = slog.Default()
	}
	// Group certificates by subject while accumulating trust.
	identityGroups := make(map[string]identityGroup)
	// Cache certifier lookups.
	certifierCache := make(map[string]wallet_settings_manager.Certifier)

	for _, cert := range certificates {
		certifierIdentityKey := cert.Certifier.ToDERHex()
		subjectIdentityKey := cert.Subject.ToDERHex()

		// Lookup and cache certifier details from trustSettings
		trustedCertifier, ok := certifierCache[certifierIdentityKey]
		if !ok {
			var found bool
			trustedCertifier, found = findCertifier(trustSettings.TrustedCertifiers, certifierIdentityKey)
			if !found {
				// Skip this certificate if its certifier is not trusted.
				continue
			}
			certifierCache[certifierIdentityKey] = trustedCertifier
		}

		// Create the IdentityCertifier object that we want to attach.
		certifierInfo := wallet.IdentityCertifier{
			Name:        trustedCertifier.Name,
			IconUrl:     to.Value(trustedCertifier.IconURL),
			Description: trustedCertifier.Description,
			//nolint:gosec
			Trust: uint8(trustedCertifier.Trust),
		}

		revealedKeyring := make(map[string]string, len(cert.Keyring))
		for k, v := range cert.Keyring {
			revealedKeyring[to.String(k)] = to.String(v)
		}

		mappedCert, err := MapVerifiableCertificateToCertificate(cert)
		if err != nil {
			// continue on failed certificate mapping but log the error
			// matches TS version where it doesn't stop on any map failure
			logger.Error("failed to map verifiable certificate to wallet.Certificate", slog.Any("error", err))
			continue
		}

		extendedCert := wallet.IdentityCertificate{
			Certificate:             mappedCert,
			DecryptedFields:         cert.DecryptedFields,
			PubliclyRevealedKeyring: revealedKeyring,
			CertifierInfo:           certifierInfo,
		}

		// Group certificates by subject
		if _, ok := identityGroups[subjectIdentityKey]; !ok {
			identityGroups[subjectIdentityKey] = identityGroup{}
		}
		group := identityGroups[subjectIdentityKey]
		group.TotalTrust += int(certifierInfo.Trust)
		group.Members = append(group.Members, extendedCert)
		identityGroups[subjectIdentityKey] = group
	}

	results := make([]wallet.IdentityCertificate, 0)
	// Filter out groups that do not meet the trust threshold and flatten the results.
	for _, group := range identityGroups {
		if group.TotalTrust >= trustSettings.TrustLevel {
			results = append(results, group.Members...)
		}
	}

	slices.SortFunc(results, func(a, b wallet.IdentityCertificate) int {
		return int(b.CertifierInfo.Trust) - int(a.CertifierInfo.Trust)
	})

	return &wallet.DiscoverCertificatesResult{
		//nolint:gosec
		TotalCertificates: uint32(len(results)),
		Certificates:      results,
	}, nil
}

func findCertifier(certifiers []wallet_settings_manager.Certifier, identityKey string) (wallet_settings_manager.Certifier, bool) {
	for _, certifier := range certifiers {
		if certifier.IdentityKey == identityKey {
			return certifier, true
		}
	}

	return wallet_settings_manager.Certifier{}, false
}
