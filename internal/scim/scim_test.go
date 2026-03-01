package scim

import (
	"context"
	"net/http/httptest"
	"testing"

	libscim "github.com/elimity-com/scim"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSCIMDB implements SCIMStore for testing.
type mockSCIMDB struct {
	createUserFn       func(ctx context.Context, arg db.CreateUserParams) (db.UserTable, error)
	getUserFn          func(ctx context.Context, userID string) (db.UserTable, error)
	listUsersFn        func(ctx context.Context) ([]db.UserTable, error)
	updateUserFn       func(ctx context.Context, arg db.UpdateUserParams) (db.UserTable, error)
	deleteUserFn       func(ctx context.Context, userID string) error
	updateUserMetaFn   func(ctx context.Context, arg db.UpdateUserMetadataParams) error
	createTeamFn       func(ctx context.Context, arg db.CreateTeamParams) (db.TeamTable, error)
	getTeamFn          func(ctx context.Context, teamID string) (db.TeamTable, error)
	listTeamsFn        func(ctx context.Context) ([]db.TeamTable, error)
	updateTeamFn       func(ctx context.Context, arg db.UpdateTeamParams) (db.TeamTable, error)
	deleteTeamFn       func(ctx context.Context, teamID string) error
	updateTeamMetaFn   func(ctx context.Context, arg db.UpdateTeamMetadataParams) error
	addTeamMemberFn    func(ctx context.Context, arg db.AddTeamMemberParams) error
	removeTeamMemberFn func(ctx context.Context, arg db.RemoveTeamMemberParams) error
	listTokensByUserFn func(ctx context.Context, userID *string) ([]db.VerificationToken, error)
	blockTokenFn       func(ctx context.Context, token string) error
}

func (m *mockSCIMDB) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.UserTable, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, arg)
	}
	return db.UserTable{UserID: arg.UserID}, nil
}
func (m *mockSCIMDB) GetUser(ctx context.Context, userID string) (db.UserTable, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, userID)
	}
	return db.UserTable{UserID: userID}, nil
}
func (m *mockSCIMDB) ListUsers(ctx context.Context) ([]db.UserTable, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx)
	}
	return []db.UserTable{}, nil
}
func (m *mockSCIMDB) UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.UserTable, error) {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, arg)
	}
	return db.UserTable{UserID: arg.UserID}, nil
}
func (m *mockSCIMDB) DeleteUser(ctx context.Context, userID string) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, userID)
	}
	return nil
}
func (m *mockSCIMDB) UpdateUserMetadata(ctx context.Context, arg db.UpdateUserMetadataParams) error {
	if m.updateUserMetaFn != nil {
		return m.updateUserMetaFn(ctx, arg)
	}
	return nil
}
func (m *mockSCIMDB) CreateTeam(ctx context.Context, arg db.CreateTeamParams) (db.TeamTable, error) {
	if m.createTeamFn != nil {
		return m.createTeamFn(ctx, arg)
	}
	return db.TeamTable{TeamID: arg.TeamID}, nil
}
func (m *mockSCIMDB) GetTeam(ctx context.Context, teamID string) (db.TeamTable, error) {
	if m.getTeamFn != nil {
		return m.getTeamFn(ctx, teamID)
	}
	return db.TeamTable{TeamID: teamID}, nil
}
func (m *mockSCIMDB) ListTeams(ctx context.Context) ([]db.TeamTable, error) {
	if m.listTeamsFn != nil {
		return m.listTeamsFn(ctx)
	}
	return []db.TeamTable{}, nil
}
func (m *mockSCIMDB) UpdateTeam(ctx context.Context, arg db.UpdateTeamParams) (db.TeamTable, error) {
	if m.updateTeamFn != nil {
		return m.updateTeamFn(ctx, arg)
	}
	return db.TeamTable{TeamID: arg.TeamID}, nil
}
func (m *mockSCIMDB) DeleteTeam(ctx context.Context, teamID string) error {
	if m.deleteTeamFn != nil {
		return m.deleteTeamFn(ctx, teamID)
	}
	return nil
}
func (m *mockSCIMDB) UpdateTeamMetadata(ctx context.Context, arg db.UpdateTeamMetadataParams) error {
	if m.updateTeamMetaFn != nil {
		return m.updateTeamMetaFn(ctx, arg)
	}
	return nil
}
func (m *mockSCIMDB) AddTeamMember(ctx context.Context, arg db.AddTeamMemberParams) error {
	if m.addTeamMemberFn != nil {
		return m.addTeamMemberFn(ctx, arg)
	}
	return nil
}
func (m *mockSCIMDB) RemoveTeamMember(ctx context.Context, arg db.RemoveTeamMemberParams) error {
	if m.removeTeamMemberFn != nil {
		return m.removeTeamMemberFn(ctx, arg)
	}
	return nil
}
func (m *mockSCIMDB) ListVerificationTokensByUser(ctx context.Context, userID *string) ([]db.VerificationToken, error) {
	if m.listTokensByUserFn != nil {
		return m.listTokensByUserFn(ctx, userID)
	}
	return nil, nil
}
func (m *mockSCIMDB) BlockVerificationToken(ctx context.Context, token string) error {
	if m.blockTokenFn != nil {
		return m.blockTokenFn(ctx, token)
	}
	return nil
}

func newMockSCIMDB() *mockSCIMDB { return &mockSCIMDB{} }

// ---- UserHandler tests ----

func TestUserHandler_Get(t *testing.T) {
	h := &UserHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("GET", "/Users/u1", nil)
	res, err := h.Get(req, "u1")
	require.NoError(t, err)
	assert.Equal(t, "u1", res.ID)
}

func TestUserHandler_Get_NotFound(t *testing.T) {
	ms := newMockSCIMDB()
	ms.getUserFn = func(_ context.Context, _ string) (db.UserTable, error) {
		return db.UserTable{}, assert.AnError
	}
	h := &UserHandler{DB: ms}
	req := httptest.NewRequest("GET", "/Users/u1", nil)
	_, err := h.Get(req, "u1")
	assert.Error(t, err)
}

func TestUserHandler_GetAll(t *testing.T) {
	ms := newMockSCIMDB()
	ms.listUsersFn = func(_ context.Context) ([]db.UserTable, error) {
		return []db.UserTable{{UserID: "u1"}, {UserID: "u2"}}, nil
	}
	h := &UserHandler{DB: ms}
	req := httptest.NewRequest("GET", "/Users", nil)
	page, err := h.GetAll(req, libscim.ListRequestParams{Count: 10, StartIndex: 1})
	require.NoError(t, err)
	assert.Equal(t, 2, page.TotalResults)
}

func TestUserHandler_Create(t *testing.T) {
	h := &UserHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("POST", "/Users", nil)
	attrs := libscim.ResourceAttributes{
		"userName": "alice",
		"emails":   []any{map[string]any{"value": "alice@example.com"}},
	}
	res, err := h.Create(req, attrs)
	require.NoError(t, err)
	assert.NotEmpty(t, res.ID)
}

func TestUserHandler_Delete(t *testing.T) {
	h := &UserHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("DELETE", "/Users/u1", nil)
	err := h.Delete(req, "u1")
	assert.NoError(t, err)
}

func TestUserHandler_Replace(t *testing.T) {
	h := &UserHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("PUT", "/Users/u1", nil)
	attrs := libscim.ResourceAttributes{
		"userName": "alice-updated",
	}
	res, err := h.Replace(req, "u1", attrs)
	require.NoError(t, err)
	assert.Equal(t, "u1", res.ID)
}

// ---- GroupHandler tests ----

func TestGroupHandler_Get(t *testing.T) {
	h := &GroupHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("GET", "/Groups/g1", nil)
	res, err := h.Get(req, "g1")
	require.NoError(t, err)
	assert.Equal(t, "g1", res.ID)
}

func TestGroupHandler_Get_NotFound(t *testing.T) {
	ms := newMockSCIMDB()
	ms.getTeamFn = func(_ context.Context, _ string) (db.TeamTable, error) {
		return db.TeamTable{}, assert.AnError
	}
	h := &GroupHandler{DB: ms}
	req := httptest.NewRequest("GET", "/Groups/g1", nil)
	_, err := h.Get(req, "g1")
	assert.Error(t, err)
}

func TestGroupHandler_GetAll(t *testing.T) {
	ms := newMockSCIMDB()
	ms.listTeamsFn = func(_ context.Context) ([]db.TeamTable, error) {
		return []db.TeamTable{{TeamID: "g1"}}, nil
	}
	h := &GroupHandler{DB: ms}
	req := httptest.NewRequest("GET", "/Groups", nil)
	page, err := h.GetAll(req, libscim.ListRequestParams{Count: 10, StartIndex: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, page.TotalResults)
}

func TestGroupHandler_Create(t *testing.T) {
	h := &GroupHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("POST", "/Groups", nil)
	attrs := libscim.ResourceAttributes{
		"displayName": "Engineering",
	}
	res, err := h.Create(req, attrs)
	require.NoError(t, err)
	assert.NotEmpty(t, res.ID)
}

func TestGroupHandler_Delete(t *testing.T) {
	h := &GroupHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("DELETE", "/Groups/g1", nil)
	err := h.Delete(req, "g1")
	assert.NoError(t, err)
}

func TestGroupHandler_Replace(t *testing.T) {
	h := &GroupHandler{DB: newMockSCIMDB()}
	req := httptest.NewRequest("PUT", "/Groups/g1", nil)
	attrs := libscim.ResourceAttributes{
		"displayName": "Infra",
	}
	res, err := h.Replace(req, "g1", attrs)
	require.NoError(t, err)
	assert.Equal(t, "g1", res.ID)
}
