package wdk_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestReviewActionErrors_JSONRoundTrip(t *testing.T) {
	src := wdk.ReviewActionErrors{
		"PostFromBEEF Validation": errors.New("boom"),
		"other":                   errors.New("other err"),
	}

	raw, err := json.Marshal(src)
	require.NoError(t, err)
	assert.JSONEq(t, `{"PostFromBEEF Validation":"boom","other":"other err"}`, string(raw))

	var dst wdk.ReviewActionErrors
	require.NoError(t, json.Unmarshal(raw, &dst))
	require.Len(t, dst, 2)
	require.EqualError(t, dst["PostFromBEEF Validation"], "boom")
	require.EqualError(t, dst["other"], "other err")
}

func TestReviewActionErrors_UnmarshalLegacyEmptyObjects(t *testing.T) {
	// encoding/json historically marshaled map[string]error as empty objects.
	legacy := []byte(`{"PostFromBEEF Validation":{}}`)
	var dst wdk.ReviewActionErrors
	require.NoError(t, json.Unmarshal(legacy, &dst))
	require.Len(t, dst, 1)
	require.Contains(t, dst, "PostFromBEEF Validation")
}

func TestReviewActionResult_JSONRoundTripWithErrors(t *testing.T) {
	src := wdk.ProcessActionResult{
		SendWithResults: []wdk.SendWithResult{
			{TxID: "aa", Status: wdk.SendWithResultStatusSending},
		},
		NotDelayedResults: []wdk.ReviewActionResult{
			{
				TxID:   "aa",
				Status: wdk.ReviewActionResultStatusServiceError,
				Errors: wdk.ReviewActionErrors{"svc": errors.New("fail")},
			},
		},
	}

	raw, err := json.Marshal(src)
	require.NoError(t, err)

	var dst wdk.ProcessActionResult
	require.NoError(t, json.Unmarshal(raw, &dst))
	require.Len(t, dst.NotDelayedResults, 1)
	assert.Equal(t, wdk.ReviewActionResultStatusServiceError, dst.NotDelayedResults[0].Status)
	require.EqualError(t, dst.NotDelayedResults[0].Errors["svc"], "fail")
}
