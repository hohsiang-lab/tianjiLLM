//go:build integration

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

func TestConcurrentAdminDisablePreservesOneActiveAdmin(t *testing.T) {
	pool, queries := setupIntegrationDB(t)
	ctx := context.Background()

	baseline, err := queries.CountUsersByRole(ctx, "proxy_admin")
	require.NoError(t, err)
	if baseline != 0 {
		t.Skipf("requires an isolated test database without active proxy_admin users; found %d", baseline)
	}

	prefix := "test-admin-race-" + uuid.NewString()
	userIDs := []string{prefix + "-1", prefix + "-2"}
	for i, userID := range userIDs {
		alias := "Race Admin"
		email := userID + "@example.com"
		_, err := queries.CreateUser(ctx, db.CreateUserParams{
			UserID:    userID,
			UserAlias: &alias,
			UserEmail: &email,
			UserRole:  "proxy_admin",
			Teams:     []string{},
			Models:    []string{},
			CreatedBy: "test",
		})
		require.NoError(t, err, "create admin %d", i+1)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM "UserTable" WHERE user_id = ANY($1)`, userIDs)
	})

	h := &UIHandler{DB: queries, Pool: pool}
	start := make(chan struct{})
	var wg sync.WaitGroup
	responses := make([]*httptest.ResponseRecorder, len(userIDs))

	for i, userID := range userIDs {
		wg.Add(1)
		go func(i int, userID string) {
			defer wg.Done()
			<-start

			form := url.Values{"return_to": {"detail"}}
			req := httptest.NewRequest(
				http.MethodPost,
				"/ui/users/"+userID+"/block",
				strings.NewReader(form.Encode()),
			)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = withChiURLParam(req, "user_id", userID)
			responses[i] = httptest.NewRecorder()
			h.handleUserBlock(responses[i], req)
		}(i, userID)
	}

	close(start)
	wg.Wait()

	for _, response := range responses {
		assert.Equal(t, http.StatusSeeOther, response.Code)
	}
	activeAdmins, err := queries.CountUsersByRole(ctx, "proxy_admin")
	require.NoError(t, err)
	assert.Equal(t, int64(1), activeAdmins)
}
