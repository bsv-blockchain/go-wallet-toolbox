package mapping

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapActionStatus_Aborted(t *testing.T) {
	got, err := mapActionStatus("aborted")
	require.NoError(t, err)
	assert.Equal(t, ActionStatusAborted, got)
}

func TestMapActionStatus_Unknown(t *testing.T) {
	_, err := mapActionStatus("bogus")
	require.Error(t, err)
}
