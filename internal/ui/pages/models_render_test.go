package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderModelForm(t *testing.T, component templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render model form: %v", err)
	}
	return buf.String()
}

func TestModelFormsOmitRemovedOpenAISubscriptionControls(t *testing.T) {
	forms := map[string]templ.Component{
		"create": createModelForm(),
		"edit": EditModelForm(ModelRow{
			ID:            "model-1",
			ModelName:     "gpt-4o",
			Provider:      "openai",
			ProviderModel: "gpt-4o",
		}),
	}

	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			html := renderModelForm(t, form)
			for _, removed := range []string{
				"openai-subscription-credentials-selector",
				"openai_subscription_credential_ids",
				"openai_subscription_transport",
				"direct_openai_http",
				"chatgpt_codex_backend",
			} {
				if strings.Contains(html, removed) {
					t.Errorf("rendered form contains removed control %q", removed)
				}
			}
			for _, retained := range []string{`name="api_base"`, `name="api_key"`} {
				if !strings.Contains(html, retained) {
					t.Errorf("rendered form lost generic field %q", retained)
				}
			}
		})
	}
}

func TestModelsSurfaceOmitsModelAccessControl(t *testing.T) {
	forms := map[string]templ.Component{
		"create": createModelForm(),
		"edit": EditModelForm(ModelRow{
			ID:            "model-1",
			ModelName:     "gpt-4o",
			Provider:      "openai",
			ProviderModel: "gpt-4o",
		}),
		"list": ModelsTable(ModelsPageData{
			Models: []ModelRow{{
				ID:            "model-1",
				ModelName:     "gpt-4o",
				Provider:      "openai",
				ProviderModel: "gpt-4o",
			}},
			DBAvailable: true,
		}),
	}

	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			html := renderModelForm(t, form)
			for _, removed := range []string{
				"Access Control",
				`name="allowed_orgs"`,
				`name="allowed_teams"`,
				`name="allowed_keys"`,
				">Access<",
			} {
				if strings.Contains(html, removed) {
					t.Errorf("rendered model surface contains removed access-control element %q", removed)
				}
			}
		})
	}
}
