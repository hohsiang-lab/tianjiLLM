package contract

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseIDSecurity_EncryptDecrypt(t *testing.T) {
	sec := middleware.NewResponseIDSecurity("test-secret-key")

	encrypted := sec.EncryptID("resp-123", "user-1", "team-1")
	require.Contains(t, encrypted, "resp-123.")

	// Same user/team should decrypt successfully
	original, valid := sec.DecryptID(encrypted, "user-1", "team-1")
	assert.True(t, valid)
	assert.Equal(t, "resp-123", original)
}

func TestResponseIDSecurity_CrossUserRejection(t *testing.T) {
	sec := middleware.NewResponseIDSecurity("test-secret-key")

	encrypted := sec.EncryptID("resp-456", "user-1", "team-1")

	// Different user should be rejected
	_, valid := sec.DecryptID(encrypted, "user-2", "team-1")
	assert.False(t, valid, "should reject access from different user")
}

func TestResponseIDSecurity_CrossTeamRejection(t *testing.T) {
	sec := middleware.NewResponseIDSecurity("test-secret-key")

	encrypted := sec.EncryptID("resp-789", "user-1", "team-1")

	// Different team should be rejected
	_, valid := sec.DecryptID(encrypted, "user-1", "team-2")
	assert.False(t, valid, "should reject access from different team")
}

func TestResponseIDSecurity_PlainIDPassThrough(t *testing.T) {
	sec := middleware.NewResponseIDSecurity("test-secret-key")

	// A plain response ID without signature should pass through
	original, valid := sec.DecryptID("resp-plain-no-dot", "user-1", "team-1")
	assert.True(t, valid, "plain IDs should pass through")
	assert.Equal(t, "resp-plain-no-dot", original)
}

func TestResponseIDSecurity_DifferentSecrets(t *testing.T) {
	sec1 := middleware.NewResponseIDSecurity("secret-1")
	sec2 := middleware.NewResponseIDSecurity("secret-2")

	encrypted := sec1.EncryptID("resp-100", "user-1", "team-1")

	// Different secret should produce different encryption
	_, valid := sec2.DecryptID(encrypted, "user-1", "team-1")
	assert.False(t, valid, "different secrets should not validate")
}

func TestNewResponseSecurityMiddleware_NilSecurity(t *testing.T) {
	mw := middleware.NewResponseSecurityMiddleware(nil)
	assert.NotNil(t, mw, "should return passthrough middleware when security is nil")
}
