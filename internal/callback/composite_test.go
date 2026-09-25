package callback

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGate7d_EqualsGateConstant(t *testing.T) {
	assert.Equal(t, gate7d, Gate7d, "exported Gate7d must equal unexported gate7d")
}
