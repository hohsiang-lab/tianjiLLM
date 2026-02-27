package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// guardrailCols are the columns returned by guardrail queries.
var guardrailCols = []string{
	"id", "guardrail_name", "guardrail_type", "config",
	"failure_policy", "enabled", "created_at", "updated_at",
}

// newGuardrailTestHandler creates a UIHandler backed by pgxmock.
func newGuardrailTestHandler(t *testing.T) (*UIHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := &UIHandler{
		DB: db.New(mock),
	}
	return h, mock
}

// withChiParam injects a chi URL param into the request context.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// postForm creates a POST request with form values.
func postForm(target string, vals url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// ---------- List ----------

func TestGuardrailList(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	rows := pgxmock.NewRows(guardrailCols).
		AddRow("g1", "block-pii", "regex", []byte(`{}`), "fail_closed", true, nil, nil).
		AddRow("g2", "rate-limit", "token_bucket", []byte(`{"rate":10}`), "fail_open", false, nil, nil)
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable"`).WillReturnRows(rows)

	r := httptest.NewRequest(http.MethodGet, "/ui/guardrails", nil)
	w := httptest.NewRecorder()
	h.handleGuardrails(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "block-pii")
	assert.Contains(t, body, "rate-limit")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGuardrailList_Empty(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	rows := pgxmock.NewRows(guardrailCols)
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable"`).WillReturnRows(rows)

	r := httptest.NewRequest(http.MethodGet, "/ui/guardrails", nil)
	w := httptest.NewRecorder()
	h.handleGuardrails(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Create ----------

func TestGuardrailCreate(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfigByName → not found
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE guardrail_name`).
		WithArgs("new-guard").
		WillReturnError(fmt.Errorf("no rows"))

	// CreateGuardrailConfig
	mock.ExpectQuery(`INSERT INTO "GuardrailConfigTable"`).
		WithArgs("new-guard", "regex", []byte(`{"pattern":"\\d+"}`), "fail_closed", true).
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g-new", "new-guard", "regex", []byte(`{"pattern":"\\d+"}`), "fail_closed", true, nil, nil))

	// loadGuardrailsPageData after create (re-list)
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g-new", "new-guard", "regex", []byte(`{"pattern":"\\d+"}`), "fail_closed", true, nil, nil))

	vals := url.Values{
		"guardrail_name": {"new-guard"},
		"guardrail_type": {"regex"},
		"failure_policy": {"fail_closed"},
		"config":         {`{"pattern":"\\d+"}`},
		"enabled":        {"on"},
	}
	r := postForm("/ui/guardrails", vals)
	w := httptest.NewRecorder()
	h.handleGuardrailCreate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "created successfully")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGuardrailCreate_DuplicateName(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfigByName → found
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE guardrail_name`).
		WithArgs("existing").
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g-exist", "existing", "regex", []byte(`{}`), "fail_open", true, nil, nil))

	// loadGuardrailsPageData for error response
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols))

	vals := url.Values{
		"guardrail_name": {"existing"},
		"guardrail_type": {"regex"},
	}
	r := postForm("/ui/guardrails", vals)
	w := httptest.NewRecorder()
	h.handleGuardrailCreate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already exists")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGuardrailCreate_InvalidJSON(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfigByName → not found
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE guardrail_name`).
		WithArgs("bad-json").
		WillReturnError(fmt.Errorf("no rows"))

	// loadGuardrailsPageData for error response
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols))

	vals := url.Values{
		"guardrail_name": {"bad-json"},
		"guardrail_type": {"regex"},
		"config":         {`{not valid json`},
	}
	r := postForm("/ui/guardrails", vals)
	w := httptest.NewRecorder()
	h.handleGuardrailCreate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "valid JSON")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Update ----------

func TestGuardrailUpdate(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfigByName → same ID (no conflict)
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE guardrail_name`).
		WithArgs("updated-name").
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "updated-name", "regex", []byte(`{}`), "fail_open", true, nil, nil))

	// UpdateGuardrailConfig
	mock.ExpectQuery(`UPDATE "GuardrailConfigTable"`).
		WithArgs("g1", "updated-name", "regex", []byte(`{"new":true}`), "fail_closed", false).
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "updated-name", "regex", []byte(`{"new":true}`), "fail_closed", false, nil, nil))

	// loadGuardrailsPageData
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "updated-name", "regex", []byte(`{"new":true}`), "fail_closed", false, nil, nil))

	vals := url.Values{
		"guardrail_name": {"updated-name"},
		"guardrail_type": {"regex"},
		"failure_policy": {"fail_closed"},
		"config":         {`{"new":true}`},
	}
	r := postForm("/ui/guardrails/g1", vals)
	r = withChiParam(r, "id", "g1")
	w := httptest.NewRecorder()
	h.handleGuardrailUpdate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "updated successfully")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGuardrailUpdate_NameConflict(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfigByName → different ID (conflict)
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE guardrail_name`).
		WithArgs("taken-name").
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g-other", "taken-name", "regex", []byte(`{}`), "fail_open", true, nil, nil))

	// loadGuardrailsPageData
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols))

	vals := url.Values{
		"guardrail_name": {"taken-name"},
		"guardrail_type": {"regex"},
	}
	r := postForm("/ui/guardrails/g1", vals)
	r = withChiParam(r, "id", "g1")
	w := httptest.NewRecorder()
	h.handleGuardrailUpdate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "already exists")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Delete ----------

func TestGuardrailDelete(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfig
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE id = \$1`).
		WithArgs("g1").
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "my-guard", "regex", []byte(`{}`), "fail_open", true, nil, nil))

	// ListPolicies → no bindings
	mock.ExpectQuery(`SELECT .+ FROM "PolicyTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "name", "parent_id", "conditions", "guardrails_add",
			"guardrails_remove", "pipeline", "description", "created_by",
			"created_at", "updated_at",
		}))

	// DeleteGuardrailConfig
	mock.ExpectExec(`DELETE FROM "GuardrailConfigTable" WHERE id`).
		WithArgs("g1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	// loadGuardrailsPageData
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols))

	r := postForm("/ui/guardrails/g1/delete", url.Values{})
	r = withChiParam(r, "id", "g1")
	w := httptest.NewRecorder()
	h.handleGuardrailDelete(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "deleted successfully")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGuardrailDelete_WithPolicyBinding(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfig
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE id = \$1`).
		WithArgs("g1").
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "my-guard", "regex", []byte(`{}`), "fail_open", true, nil, nil))

	// ListPolicies → one policy references this guardrail
	mock.ExpectQuery(`SELECT .+ FROM "PolicyTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "name", "parent_id", "conditions", "guardrails_add",
			"guardrails_remove", "pipeline", "description", "created_by",
			"created_at", "updated_at",
		}).AddRow("p1", "block-policy", nil, []byte(`{}`), []string{"my-guard"},
			[]string{}, nil, nil, nil, nil, nil))

	// loadGuardrailsPageData (for error toast render)
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "my-guard", "regex", []byte(`{}`), "fail_open", true, nil, nil))

	r := postForm("/ui/guardrails/g1/delete", url.Values{})
	r = withChiParam(r, "id", "g1")
	w := httptest.NewRecorder()
	h.handleGuardrailDelete(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Cannot delete")
	assert.Contains(t, w.Body.String(), "block-policy")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Toggle ----------

func TestGuardrailToggle(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	// GetGuardrailConfig → currently enabled
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE id = \$1`).
		WithArgs("g1").
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "my-guard", "regex", []byte(`{}`), "fail_open", true, nil, nil))

	// UpdateGuardrailConfig with enabled=false
	mock.ExpectQuery(`UPDATE "GuardrailConfigTable"`).
		WithArgs("g1", "my-guard", "regex", []byte(`{}`), "fail_open", false).
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "my-guard", "regex", []byte(`{}`), "fail_open", false, nil, nil))

	// loadGuardrailsPageData
	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" ORDER BY`).
		WillReturnRows(pgxmock.NewRows(guardrailCols).
			AddRow("g1", "my-guard", "regex", []byte(`{}`), "fail_open", false, nil, nil))

	r := postForm("/ui/guardrails/g1/toggle", url.Values{})
	r = withChiParam(r, "id", "g1")
	w := httptest.NewRecorder()
	h.handleGuardrailToggle(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "disabled")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Test Guardrail ----------

func TestGuardrailTest_EmptyInput(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	vals := url.Values{"test_text": {""}}
	r := postForm("/ui/guardrails/g1/test", vals)
	r = withChiParam(r, "id", "g1")
	w := httptest.NewRecorder()
	h.handleGuardrailTest(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "required")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGuardrailTest_NotFound(t *testing.T) {
	h, mock := newGuardrailTestHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT .+ FROM "GuardrailConfigTable" WHERE id = \$1`).
		WithArgs("g-missing").
		WillReturnError(fmt.Errorf("no rows"))

	vals := url.Values{"test_text": {"hello world"}}
	r := postForm("/ui/guardrails/g-missing/test", vals)
	r = withChiParam(r, "id", "g-missing")
	w := httptest.NewRecorder()
	h.handleGuardrailTest(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
	require.NoError(t, mock.ExpectationsWereMet())
}
