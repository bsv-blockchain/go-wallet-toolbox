//go:build pkcs11

package pkcs11

import (
	"testing"

	"github.com/miekg/pkcs11"
	"github.com/stretchr/testify/require"
)

func TestLoginPreservesExistingUserSession(t *testing.T) {
	for _, state := range []uint{pkcs11.CKS_RO_USER_FUNCTIONS, pkcs11.CKS_RW_USER_FUNCTIONS} {
		module := &loginStub{state: state, loginErr: pkcs11.Error(pkcs11.CKR_USER_ALREADY_LOGGED_IN)}
		require.NoError(t, loginUser(module, 1, []byte("test-pin")))
		require.Zero(t, module.logins, "redundant login can invalidate every live object handle")
	}
}

func TestLoginPublicSession(t *testing.T) {
	for _, result := range []error{nil, pkcs11.Error(pkcs11.CKR_USER_ALREADY_LOGGED_IN), pkcs11.Error(pkcs11.CKR_PIN_INCORRECT)} {
		module := &loginStub{state: pkcs11.CKS_RW_PUBLIC_SESSION, loginErr: result}
		err := loginUser(module, 1, []byte("test-pin"))
		if result == pkcs11.Error(pkcs11.CKR_PIN_INCORRECT) {
			require.ErrorIs(t, err, result)
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, 1, module.logins)
	}
}

func TestLoginStopsAfterSessionInfoFailure(t *testing.T) {
	module := &loginStub{infoErr: pkcs11.Error(pkcs11.CKR_SESSION_HANDLE_INVALID)}
	require.ErrorIs(t, loginUser(module, 1, []byte("test-pin")), module.infoErr)
	require.Zero(t, module.logins)
}

type loginStub struct {
	state    uint
	infoErr  error
	loginErr error
	logins   int
}

func (m *loginStub) GetSessionInfo(pkcs11.SessionHandle) (pkcs11.SessionInfo, error) {
	return pkcs11.SessionInfo{State: m.state}, m.infoErr
}

func (m *loginStub) Login(pkcs11.SessionHandle, uint, string) error {
	m.logins++
	return m.loginErr
}
