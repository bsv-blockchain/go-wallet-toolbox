package bhs

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

const apiPrefix = "api"
const apiVersion = "v1"

func buildURL(baseURL string, segments ...string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("error parsing base URL %s: %w", baseURL, err)
	}

	basePath := strings.TrimSuffix(u.Path, "/")
	fullPath := path.Join(append([]string{basePath}, segments...)...)
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}
	u.Path = fullPath

	return u.String(), nil
}

// tipLongestURL returns "<base>/chain/tip/longest"
func tipLongestURL(baseURL string) (string, error) {
	return buildURL(baseURL, apiPrefix, apiVersion, "chain", "tip", "longest")
}

func headerByHeight(baseURL string) (string, error) {
	return buildURL(baseURL, apiPrefix, apiVersion, "chain", "header", "byHeight")
}

func bearerHeader(key string) string {
	if key == "" {
		return "" // makes IfNotEmpty a no-op
	}
	return "Bearer " + key
}

// verifyMerkleRootURL returns "<base>/chain/merkleroot/verify"
func verifyMerkleRootURL(baseURL string) (string, error) {
	return buildURL(baseURL, apiPrefix, apiVersion, "chain", "merkleroot", "verify")
}
