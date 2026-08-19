package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	clients "github.com/bsv-blockchain/go-sdk/auth/clients/authhttp"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// allowUnauthenticated lets requests reach the handlers without a BRC-103
// handshake, so that a handler's own authentication check is what is under test
// rather than the auth middleware in front of it.
func allowUnauthenticated(opt *storage.ServerOptions) {
	opt.AllowUnauthenticated = true
}

// These tests are regressions for a cross-tenant authorization bypass: the sync
// routes took the identity of *whose data to touch* from the request body
// (args.identityKey) without binding it to the authenticated BRC-103/104 peer.
// Any authenticated caller could therefore name another user's identity key and
// export (getSyncChunk) or overwrite (processSyncChunk) that user's wallet data.
//
// Both remoting surfaces are covered, because they have independent handlers:
//   - the REST adapter at /storage/v1/* (pkg/storage/v1adapter)
//   - the legacy JSON-RPC endpoint at POST / (pkg/storage/rpcserver), which is
//     what the shipped TypeScript StorageClient/StorageMobile actually talk to.

// syncArgsFor builds sync-chunk args naming victim as the user whose data is touched.
func syncArgsFor(t *testing.T, storageIdentityKey string, victim testusers.User) wdk.RequestSyncChunkArgs {
	t.Helper()
	return wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: storageIdentityKey,
		ToStorageIdentityKey:   storageIdentityKey,
		IdentityKey:            victim.IdentityKey(t),
		MaxRoughSize:           10000,
		MaxItems:               100,
		Offsets:                []wdk.SyncOffsets{},
	}
}

// TestRESTSyncRoutesRejectCrossTenantAccess covers the /storage/v1/* adapter.
func TestRESTSyncRoutesRejectCrossTenantAccess(t *testing.T) {
	t.Run("getSyncChunk cannot export another user's data", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider()

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		// The provider must never be reached with Bob's identity.
		mockStorage.EXPECT().GetSyncChunk(gomock.Any(), gomock.Any()).Times(0)

		// when: Alice asks for Bob's sync chunk
		chunk, err := client.GetSyncChunk(t.Context(), syncArgsFor(t, given.StorageIdentityKey(), testusers.Bob))

		// then:
		require.ErrorContains(t, err, "identityKey does not match authentication")
		assert.Nil(t, chunk)
	})

	t.Run("getSyncChunk still serves the caller's own data", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider()

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		aliceKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			GetSyncChunk(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
				assert.Equal(t, aliceKey, args.IdentityKey)
				return wdk.NewSyncChunk(args.FromStorageIdentityKey, args.ToStorageIdentityKey, args.IdentityKey), nil
			})

		// when: Alice asks for her own sync chunk
		chunk, err := client.GetSyncChunk(t.Context(), syncArgsFor(t, given.StorageIdentityKey(), testusers.Alice))

		// then:
		require.NoError(t, err)
		require.NotNil(t, chunk)
		assert.Equal(t, aliceKey, chunk.UserIdentityKey)
	})

	t.Run("getSyncChunk binds a blank identityKey to the caller", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider()

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		aliceKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			GetSyncChunk(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
				// A blank identityKey must not fall through to the provider as blank,
				// where it would resolve to whichever user has no identity recorded.
				assert.Equal(t, aliceKey, args.IdentityKey)
				return wdk.NewSyncChunk(args.FromStorageIdentityKey, args.ToStorageIdentityKey, args.IdentityKey), nil
			})

		args := syncArgsFor(t, given.StorageIdentityKey(), testusers.Alice)
		args.IdentityKey = ""

		// when:
		_, err := client.GetSyncChunk(t.Context(), args)

		// then: assertions happen inside the mock
		require.NoError(t, err)
	})

	t.Run("findOrInsertSyncStateAuth uses the caller's identity, not the supplied AuthID", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		client, cleanupCli := given.RPCClientForUser(testusers.Alice)
		defer cleanupCli()

		aliceKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			FindOrInsertSyncStateAuth(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, auth wdk.AuthID, _, _ string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
				assert.Equal(t, aliceKey, auth.IdentityKey)
				assert.NotEqual(t, testusers.Bob.IdentityKey(t), auth.IdentityKey)
				return &wdk.FindOrInsertSyncStateAuthResponse{SyncState: &wdk.TableSyncState{}, IsNew: true}, nil
			})

		// when: Alice passes Bob's AuthID
		_, err := client.FindOrInsertSyncStateAuth(t.Context(), testusers.Bob.AuthID(), given.StorageIdentityKey(), "backup-storage")

		// then: assertions happen inside the mock
		require.NoError(t, err)
	})

	t.Run("setActive uses the caller's identity, not the supplied AuthID", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider().WithDefaultFindOrInsertUser(t)

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		aliceKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			SetActive(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, auth wdk.AuthID, _ string) error {
				assert.Equal(t, aliceKey, auth.IdentityKey)
				assert.NotEqual(t, testusers.Bob.IdentityKey(t), auth.IdentityKey)
				return nil
			})

		// The V1 client's SetActive is a local no-op, so drive the route directly.
		status, body := postAuthenticated(t, given.ServerURL(), testusers.Alice, "/storage/v1/sync/active",
			map[string]string{"newActiveStorageIdentityKey": given.StorageIdentityKey()})

		require.Equal(t, http.StatusOK, status, "body: %s", body)
	})

	t.Run("migrate rejects unauthenticated callers", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider()

		cleanupSrv := given.StartedRPCServerFor(mockStorage, allowUnauthenticated)
		defer cleanupSrv()

		// Migrate reinitializes storage-wide settings and must never run unauthenticated.
		mockStorage.EXPECT().Migrate(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		status, body := postUnauthenticated(t, given.ServerURL(), "/storage/v1/migrate",
			map[string]string{"storageName": "attacker-storage", "storageIdentityKey": given.StorageIdentityKey()})

		assert.Equal(t, http.StatusUnauthorized, status, "body: %s", body)
	})
}

// TestJSONRPCSyncMethodsRejectCrossTenantAccess covers the legacy JSON-RPC
// endpoint at POST /, the surface the TypeScript StorageClient talks to.
func TestJSONRPCSyncMethodsRejectCrossTenantAccess(t *testing.T) {
	t.Run("getSyncChunk cannot export another user's data", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider()

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		mockStorage.EXPECT().GetSyncChunk(gomock.Any(), gomock.Any()).Times(0)

		// when: Alice asks for Bob's sync chunk over JSON-RPC
		resp := callJSONRPC(t, given.ServerURL(), testusers.Alice, "getSyncChunk",
			[]any{syncArgsFor(t, given.StorageIdentityKey(), testusers.Bob)})

		// then:
		require.NotNil(t, resp.Error, "expected a JSON-RPC error, got result: %s", string(resp.Result))
		assert.Contains(t, resp.Error.Message, "identityKey does not match authentication")
	})

	t.Run("processSyncChunk cannot overwrite another user's data", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider()

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		mockStorage.EXPECT().ProcessSyncChunk(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		storageKey := given.StorageIdentityKey()
		chunk := wdk.NewSyncChunk(storageKey, storageKey, testusers.Bob.IdentityKey(t))

		// when: Alice pushes a chunk of data at Bob's account
		resp := callJSONRPC(t, given.ServerURL(), testusers.Alice, "processSyncChunk",
			[]any{syncArgsFor(t, storageKey, testusers.Bob), chunk})

		// then:
		require.NotNil(t, resp.Error, "expected a JSON-RPC error, got result: %s", string(resp.Result))
		assert.Contains(t, resp.Error.Message, "identityKey does not match authentication")
	})

	t.Run("getSyncChunk still serves the caller's own data", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		mockStorage := given.MockProvider()

		cleanupSrv := given.StartedRPCServerFor(mockStorage)
		defer cleanupSrv()

		aliceKey := testusers.Alice.IdentityKey(t)

		mockStorage.EXPECT().
			GetSyncChunk(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
				assert.Equal(t, aliceKey, args.IdentityKey)
				return wdk.NewSyncChunk(args.FromStorageIdentityKey, args.ToStorageIdentityKey, args.IdentityKey), nil
			})

		// when:
		resp := callJSONRPC(t, given.ServerURL(), testusers.Alice, "getSyncChunk",
			[]any{syncArgsFor(t, given.StorageIdentityKey(), testusers.Alice)})

		// then:
		require.Nil(t, resp.Error, "unexpected JSON-RPC error: %+v", resp.Error)
	})
}

// --- helpers ---

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type jsonRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *jsonRPCError   `json:"error"`
}

// callJSONRPC posts a BRC-103 authenticated JSON-RPC request to the server root,
// which is where rpcserver registers the legacy handler.
func callJSONRPC(t *testing.T, serverURL string, user testusers.User, method string, params []any) jsonRPCResponse {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	require.NoError(t, err)

	status, raw := postRawAuthenticated(t, serverURL, user, "/", body)
	require.Equal(t, http.StatusOK, status, "JSON-RPC transport error, body: %s", raw)

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(raw, &resp), "body: %s", raw)
	return resp
}

// postAuthenticated posts a JSON payload authenticated as user via BRC-103.
func postAuthenticated(t *testing.T, serverURL string, user testusers.User, path string, payload any) (int, []byte) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return postRawAuthenticated(t, serverURL, user, path, body)
}

func postRawAuthenticated(t *testing.T, serverURL string, user testusers.User, path string, body []byte) (int, []byte) {
	t.Helper()

	protoWallet, err := wallet.NewCompletedProtoWallet(user.PrivateKey(t))
	require.NoError(t, err)

	authFetch := clients.New(protoWallet)

	resp, err := authFetch.Fetch(t.Context(), serverURL+path, &clients.SimplifiedFetchRequestOptions{
		Method:  http.MethodPost,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

// postUnauthenticated posts a plain JSON payload with no BRC-103 handshake.
func postUnauthenticated(t *testing.T, serverURL, path string, payload any) (int, []byte) {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, serverURL+path, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}
