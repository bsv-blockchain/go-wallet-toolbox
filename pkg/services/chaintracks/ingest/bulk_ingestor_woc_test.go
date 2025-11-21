package ingest

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/stretchr/testify/require"
)

func TestBulkIngestorWOC_Synchronize(t *testing.T) {
	t.Skip("This test gets actual data from WOC - use this only for manual testing purposes")
	service := NewBulkIngestorWOC(logging.NewTestLogger(t), defs.NetworkMainnet)

	presentHeight := uint(923537)
	rangeToLoad := presentHeight - 4000
	fileInfo, _, err := service.Synchronize(t.Context(), presentHeight, models.NewHeightRange(rangeToLoad, presentHeight))

	require.NoError(t, err)
	t.Logf("Fetched file info: %+v", fileInfo)
}
