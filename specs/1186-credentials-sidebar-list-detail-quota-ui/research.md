# Research: Credentials sidebar list/detail quota UI

## Repo Reality

### UI architecture

- `internal/ui/routes.go` mounts protected UI pages inside `r.Group` with `h.sessionAuth`。
- `internal/ui/pages/layout.templ` defines `AppLayout` and sidebar nav items with `navItem(href, label, iconName, activePath)`。
- Existing list/detail pages use Go `templ` components, not a client SPA framework。
- `keys.templ` provides table/filter/list patterns; `key_detail.templ` provides header, badge, card, progress bar, tabs, and back-link patterns。
- E2E helpers in `test/e2e/helpers_test.go` already provide login, navigation, DB seeding, text/count helpers, and Playwright locators。

### Credential API and metadata

- `/credentials` API routes are protected by existing proxy auth/RBAC in `internal/proxy/server.go` and `internal/auth/rbac.go`。
- `CredentialTable` has `credential_id`、`credential_name`、`credential_type`、`credential_value`、`credential_info`、organization/timestamp fields。
- `CredentialTypeOpenAISubscription = "openai_subscription"` exists in `internal/proxy/handler/credentials.go`。
- `OpenAISubscriptionCredentialInfo` safe fields include `Email`、`Scopes`、`Status`、`LastRefreshAt`、`LastError`、`DisabledReason`。
- Existing redaction helpers remove secret fields and redact secret-looking strings before API responses。

### Quota/rate-limit state

- `callback.OpenAIQuotaState` has status and two dimensions: requests and tokens。
- Each dimension has known flags for limit、remaining、reset、utilization。
- `OpenAIQuotaState.Normalize` clears expired gates and preserves unknown values distinctly from zero。
- `UIHandler` already has `RateLimitStore callback.RateLimitStore`，so UI can read stored state without a new backend API。

## External Evidence

### Context7: `templ`

Context7 `/a-h/templ` shows `templ` components can render directly in Go HTTP handlers with `Component.Render(r.Context(), w)` or `templ.Handler(...)`。This matches the repo's existing `render(ctx, w, component)` helper and supports keeping HO-1186 server-rendered.

Decision: use existing server-rendered `templ` pages; do not add a client-side framework.

### Context7: `go-chi`

Context7 `/go-chi/docs` shows `chi.Router` supports grouped routes, nested subrouters, middleware via `r.Use`, and method-specific route handlers such as `r.Get` / `r.Post`。This matches `internal/ui/routes.go` and supports adding `/ui/credentials` inside the existing protected group.

Decision: add UI routes in the current `RegisterRoutes` structure.

### GitHub grep.app prior art

Queries run:

- `@sidebar.MenuButton` with Go language filter: no useful public results。
- `templ AppLayout` with Go language filter: no useful public results。
- `x-ratelimit-remaining-requests` with Go language filter: no useful public results。

Decision: repo-local Tianji patterns are stronger evidence than public prior art for this UI.

### Official OpenAI docs

Relevant official documentation used for sibling behavior boundaries:

- API authentication uses Bearer tokens: `https://developers.openai.com/api/reference/overview#authentication`
- Models list endpoint: `https://developers.openai.com/api/reference/models/list`
- Rate limits guide: `https://platform.openai.com/docs/guides/rate-limits`

Decision: HO-1186 UI must not call OpenAI directly. It should present stored metadata/quota state produced by sibling backend features and clearly render unknown quota as unknown.

## Decisions

### Decision 1: Read-only OpenAI subscription credential UI

Chosen: list and detail only for `credential_type=openai_subscription`。

Why: Linear goal says "viewing OpenAI subscription credentials and quota/status"; create/update/delete/test/refresh/disable UI is not in scope and existing API siblings own those contracts.

Rejected:

- Build full generic credential admin UI: too broad and risks secret display.
- Include generic API-key credentials in list: distracts from OpenAI quota/status and has different metadata semantics.

### Decision 2: Use DB + RateLimitStore directly from UI handler

Chosen: UI handler reads `CredentialTable` through existing DB methods and quota through `UIHandler.RateLimitStore`。

Why: Existing UI handlers already read DB directly. A new HTTP API proxy layer would duplicate redaction and introduce an avoidable auth boundary.

Rejected:

- Have UI call `/credentials/list` over HTTP: unnecessary internal round trip and harder to test in current UI pattern.
- Decrypt token bundle to derive account metadata: violates safe metadata boundary.

### Decision 3: Narrow safe metadata view model

Chosen: decode only known safe fields into a typed metadata struct for templates。

Why: Rendering arbitrary `credential_info` can leak secret-looking strings if future writes bypass sanitization. The UI should have its own narrow egress boundary.

Rejected:

- Render raw metadata JSON: unsafe.
- Trust only API-layer redaction: UI reads DB directly, so it needs its own display guard.

### Decision 4: Separate credential health and quota gate display

Chosen: show credential health status from safe metadata and quota status from `OpenAIQuotaState` as related but distinct UI elements。

Why: Disabled credential metadata and stale quota state can disagree. Operators need both facts without one hiding the other.

Rejected:

- Single merged status only: hides important context.
- Quota-only status badge: disabled/refresh failure metadata could disappear.

### Decision 5: Unknown quota is first-class

Chosen: unknown fields render as unknown/not recorded; progress bars appear only when utilization is known。

Why: HO-1181 explicitly models unknown values with known flags. Showing 0% would falsely imply healthy unused capacity.

Rejected:

- Treat missing quota as 0 usage: misleading.
- Estimate monthly subscription quota: explicitly out of scope.
