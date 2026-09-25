# Research: Org detail OpenAI credential panel

## R-001: Data source for org-scoped credentials

Decision: Use existing sqlc-backed `ListCredentialsByOrg(ctx, &orgID)` and filter `credential_type = "openai_subscription"` in UI adapter.

Evidence:
- `internal/db/queries/credential.sql` defines `ListCredentialsByOrg` as `WHERE organization_id = $1 ORDER BY created_at DESC`。
- `internal/db/interface.go` exposes `ListCredentialsByOrg(ctx context.Context, organizationID *string)` on `db.Store`。
- This avoids loading all credentials and reduces risk of cross-org leakage。

## R-002: Credential row formatting

Decision: Reuse or extract existing Credentials UI safe row builder logic from `internal/ui/handler_credentials.go`.

Evidence:
- HO-1186 implementation already builds `pages.CredentialRow` from `CredentialTable`, safe parsed metadata, and `RateLimitStore`。
- `pages.CredentialRow` already contains the exact panel-level fields needed by HO-1189: ID, Name, Email, OrganizationID, CredentialStatus, QuotaStatus, QuotaSummary, LastRefresh, LastError。
- Duplicating parser/status helpers in org handler would create drift and secret-boundary risk。

## R-003: Page placement

Decision: Place the new panel after overview cards and before Members on Org detail.

Evidence:
- Current `orgs.templ` order is header, overview cards, Members, Teams, Metadata。
- OpenAI credentials are org-level operational resources, closer to budget/rate-limit overview than membership editing。
- Keeping it before Members makes credential status scannable without duplicating lifecycle controls。

## R-004: Security boundary

Decision: Treat Org detail as another secret egress boundary and require DOM negative assertions.

Evidence:
- `CredentialTable.credential_value` stores encrypted secret material and must never be rendered。
- Existing HO-1186 E2E has `TestCredentialDetail_SecretMaterialNotRendered`; HO-1189 needs the same guarantee on Org detail because it introduces another render path。
- `credential_info` may contain `last_error` or accidental token-looking strings; output must pass existing redaction before display。

## R-005: External docs / prior art

Decision: No new framework or external OpenAI API behavior is needed; repo-local patterns dominate.

Evidence:
- Context7 `/a-h/templ` documents rendering dynamic templ components from Go HTTP handlers with `Render(r.Context(), w)` and dynamic attributes/content。
- `go-chi/chi` docs confirm route groups/middleware/named params fit the current `/ui` protected route structure。
- Web/GitHub examples for Go templ + HTMX admin dashboards are generic; none provide a more relevant pattern than Tianji's existing `AppLayout`, `card`, `table`, `badge`, and E2E fixtures。
