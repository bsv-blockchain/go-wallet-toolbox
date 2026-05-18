package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerCORSConfigDefaultsToOpenOrigins(t *testing.T) {
	server := &Server{}

	config, ok := server.corsConfig()

	require.True(t, ok)
	require.True(t, config.Enabled)
	require.True(t, config.AllowAllOrigins)
	require.True(t, config.AllowPrivateNetwork)
}

func TestServerCORSConfigCanBeDisabled(t *testing.T) {
	server := &Server{
		options: ServerOptions{DisableCORS: true},
	}

	_, ok := server.corsConfig()

	require.False(t, ok)
}
