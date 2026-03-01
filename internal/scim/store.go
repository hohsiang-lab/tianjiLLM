package scim

import (
	"context"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// SCIMStore is the minimal DB interface required by SCIM handlers.
// *db.Queries satisfies this interface.
type SCIMStore interface {
	// Users
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.UserTable, error)
	GetUser(ctx context.Context, userID string) (db.UserTable, error)
	ListUsers(ctx context.Context) ([]db.UserTable, error)
	UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.UserTable, error)
	DeleteUser(ctx context.Context, userID string) error
	UpdateUserMetadata(ctx context.Context, arg db.UpdateUserMetadataParams) error

	// Teams
	CreateTeam(ctx context.Context, arg db.CreateTeamParams) (db.TeamTable, error)
	GetTeam(ctx context.Context, teamID string) (db.TeamTable, error)
	ListTeams(ctx context.Context) ([]db.TeamTable, error)
	UpdateTeam(ctx context.Context, arg db.UpdateTeamParams) (db.TeamTable, error)
	DeleteTeam(ctx context.Context, teamID string) error
	UpdateTeamMetadata(ctx context.Context, arg db.UpdateTeamMetadataParams) error
	AddTeamMember(ctx context.Context, arg db.AddTeamMemberParams) error
	RemoveTeamMember(ctx context.Context, arg db.RemoveTeamMemberParams) error

	// Tokens
	ListVerificationTokensByUser(ctx context.Context, userID *string) ([]db.VerificationToken, error)
	BlockVerificationToken(ctx context.Context, token string) error
}

// Compile-time check.
var _ SCIMStore = (*db.Queries)(nil)
