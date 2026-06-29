package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenLimitsSQLiteToSingleConnection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	stats := store.DB().Stats()
	require.Equal(t, 1, stats.MaxOpenConnections)
}
