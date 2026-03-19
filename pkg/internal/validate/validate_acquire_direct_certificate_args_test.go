package validate_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
)

func TestValidateAcquireDirectCertificateArgs(t *testing.T) {
	t.Run("should not return an error when given the appropriate arguments list", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.NoError(t, err)
	})

	t.Run("should not return an error when an invalid protocol type is provided", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.AcquisitionProtocol = wallet.AcquisitionProtocolIssuance

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})

	t.Run("should not return an error when a certifier is missing from the arguments list", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.Certifier = nil

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})

	t.Run("should not return an error when is missing from the arguments list", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.SerialNumber = nil

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})

	t.Run("should not return an error when a signature is missing from the arguments list", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.Signature = nil

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})

	t.Run("should not return an error when a revocation outpoint is missing from the arguments list", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.RevocationOutpoint = nil

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})

	t.Run("should not return an error when a keyring revealer is missing from the arguments list", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.KeyringRevealer = nil

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})

	t.Run("should not return an error when a keyring for subject is missing from the arguments list", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.KeyringForSubject = nil

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})

	t.Run("should not return an error when a privileged reason is an empty string, and privileged flag is set to true", func(t *testing.T) {
		// given:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.Privileged = to.Ptr(true)
		args.PrivilegedReason = ""

		// when:
		err := validate.ValidateAcquireDirectCertificateArgs(&args)

		// then:
		require.Error(t, err)
	})
}
