package callback

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_Names(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockLogger{})
	names := r.Names()
	assert.Len(t, names, 1)
	assert.Equal(t, "mockLogger", names[0])
}
