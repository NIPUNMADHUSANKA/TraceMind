package worker

import (
	"os"
	"testing"
	"tracemind/internal/store"

	"github.com/stretchr/testify/require"
)

func newWorkerTestPostgresStore(t *testing.T) (store.PostgresStore, func()) {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required for worker tests with PostgresStore")
	}

	ps, err := store.NewPostgresStore(dsn)
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, ps.Close())
	}
	return *ps, cleanup
}
