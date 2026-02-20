package scim

import (
	"net/http"

	libscim "github.com/elimity-com/scim"
	"github.com/elimity-com/scim/optional"
	"github.com/elimity-com/scim/schema"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// Config holds SCIM server configuration.
type Config struct {
	DB         *db.Queries
	UpsertUser bool // auto-create missing users on group member add
}

// NewSCIMServer creates a SCIM 2.0 http.Handler with User and Group resource types.
func NewSCIMServer(cfg Config) (http.Handler, error) {
	userHandler := &UserHandler{DB: cfg.DB}
	groupHandler := &GroupHandler{DB: cfg.DB, UpsertUser: cfg.UpsertUser}

	server, err := libscim.NewServer(
		&libscim.ServerArgs{
			ServiceProviderConfig: &libscim.ServiceProviderConfig{
				SupportPatch:     true,
				SupportFiltering: true,
				MaxResults:       200,
				AuthenticationSchemes: []libscim.AuthenticationScheme{
					{
						Type:        libscim.AuthenticationTypeOauthBearerToken,
						Name:        "OAuth Bearer Token",
						Description: "Authentication using Bearer token",
						Primary:     true,
					},
				},
			},
			ResourceTypes: []libscim.ResourceType{
				{
					ID:          optional.NewString("User"),
					Name:        "User",
					Endpoint:    "/Users",
					Description: optional.NewString("TianjiLLM User Account"),
					Schema:      schema.CoreUserSchema(),
					SchemaExtensions: []libscim.SchemaExtension{
						{
							Schema:   schema.ExtensionEnterpriseUser(),
							Required: false,
						},
					},
					Handler: userHandler,
				},
				{
					ID:          optional.NewString("Group"),
					Name:        "Group",
					Endpoint:    "/Groups",
					Description: optional.NewString("TianjiLLM Team/Group"),
					Schema:      schema.CoreGroupSchema(),
					Handler:     groupHandler,
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return server, nil
}
