package api

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDependencies(t *testing.T) {
	fixture, err := os.Open(filepath.Join("testdata", "react-16.13.0.json"))
	require.Nil(t, err)
	var fixtureObj *NpmPackageVersion
	require.Nil(t, json.NewDecoder(fixture).Decode(&fixtureObj))

	dependencies, err := resolveDependencies("react", "16.13.0")
	require.NoError(t, err)
	assert.Equal(t, fixtureObj, dependencies)
}
