package spend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockStorage struct {
	uploads []string
}

func (m *mockStorage) Name() string { return "mock" }
func (m *mockStorage) Upload(_ context.Context, key string, _ []byte) (string, error) {
	m.uploads = append(m.uploads, key)
	return "mock://" + key, nil
}

func TestStorageBackendInterface(t *testing.T) {
	var _ StorageBackend = &mockStorage{}
	var _ StorageBackend = &S3Backend{}
}

func TestMockStorageUpload(t *testing.T) {
	m := &mockStorage{}
	loc, err := m.Upload(context.Background(), "test/key.json", []byte(`{"test":true}`))
	assert.NoError(t, err)
	assert.Equal(t, "mock://test/key.json", loc)
	assert.Len(t, m.uploads, 1)
}

func TestArchiverDefaults(t *testing.T) {
	a := &Archiver{}
	assert.Equal(t, int32(0), a.BatchSize) // default, resolved to 10000 at runtime
}
