# Admin UI social authentication

TianjiLLM can authenticate admin-dashboard users through GitHub or Discord
without sending the proxy master key through a browser login form.

This feature does not add a separate identity service:

- OAuth authorization code exchange and PKCE use `golang.org/x/oauth2`.
- Goth adapters normalize the GitHub and Discord user profiles.
- SCS stores the TianjiLLM UI session server-side in PostgreSQL.
- `UserIdentityTable` maps a provider subject to a local TianjiLLM user.
- Local TianjiLLM roles determine dashboard permissions.
- Provider access and refresh tokens are used only during the callback and are
  never written to the UI session.

## Configuration

```yaml
general_settings:
  master_key: os.environ/PROXY_MASTER_KEY
  database_url: os.environ/DATABASE_URL
  ui_session_secure: true

  social_auth:
    enabled: true
    public_base_url: https://tianji.example.com
    allow_verified_email_link: false
    master_key_login_enabled: false

    github:
      client_id: os.environ/TIANJI_GITHUB_CLIENT_ID
      client_secret: os.environ/TIANJI_GITHUB_CLIENT_SECRET

    discord:
      client_id: os.environ/TIANJI_DISCORD_CLIENT_ID
      client_secret: os.environ/TIANJI_DISCORD_CLIENT_SECRET
```

When social authentication is enabled:

- `database_url` and an absolute `public_base_url` origin are required.
- `public_base_url` must use HTTPS; HTTP is accepted only for `localhost`,
  `127.0.0.1`, or `::1` development callbacks.
- At least one provider must have both `client_id` and `client_secret`.
- Master-key UI login defaults to disabled unless
  `master_key_login_enabled: true` is explicitly configured.
- `ui_session_secure` should remain `true` for HTTPS deployments.

The master key still authenticates proxy API requests. The
`master_key_login_enabled` setting controls only the admin UI login form.

Do not commit provider client secrets to the repository. Use
`os.environ/VARIABLE_NAME` references and inject the values through the
deployment secret mechanism.

## Provider callback URLs and scopes

Register the exact callback URLs derived from `public_base_url`:

| Provider | Callback URL | Requested scopes |
| --- | --- | --- |
| GitHub | `https://tianji.example.com/auth/github/callback` | `read:user`, `user:email` |
| Discord | `https://tianji.example.com/auth/discord/callback` | `identify`, `email` |

Both flows use a fresh, one-time state value and an S256 PKCE verifier. OAuth
state is stored in the server-side SCS session, expires after 10 minutes, and
is consumed before the authorization code is exchanged.

## User provisioning and identity linking

Social login is not open self-signup. A provider identity can authenticate only
when it is already linked to a local TianjiLLM user or when an administrator
has authorized an enrollment path.

The recommended path is per-user, one-time enrollment:

1. Create the local user with a unique email address and dashboard role.
2. Open the user's **Sign-in Access** card in the admin UI.
3. Enable **Next verified provider enrollment**.
4. Ask the user to sign in with GitHub or Discord.
5. The verified provider email must uniquely match the active local user.
6. After the identity is linked, enrollment is automatically disabled.

An explicit enrollment may link the user's first provider or add another
provider to an existing user. It does not allow a provider identity with a
different email to claim the account.

`allow_verified_email_link` is a global bootstrap option. When it is enabled, a
provider identity can create the first link only when all of these conditions
are true:

1. `allow_verified_email_link` is enabled.
2. The provider reports a verified email address.
3. Exactly one active TianjiLLM user has that email address.
4. That TianjiLLM user has no existing provider identities.

The stable identity key is `(provider, provider_user_id)`. Email is used only
for the guarded first-link workflow and is never the long-term identity key.
There is no self-service linking endpoint.

Administrators can review and unlink provider identities from the same
**Sign-in Access** card. Unlinking an identity increments the user's
`auth_version`, invalidating all of that user's existing UI sessions. The UI
prevents an administrator from unlinking their own last identity and prevents
removal of the last identity of the last active `proxy_admin`.

GitHub linking requires the primary email to be verified. Discord linking
requires the user object to report `verified: true`.

## Dashboard roles

The local `UserTable.user_role` value is reloaded on every authenticated
request. Sessions do not permanently cache authorization.

| Role | Dashboard access |
| --- | --- |
| `proxy_admin` | Full dashboard access, including users and credentials |
| `internal_user` | Operational management excluding users and provider credentials |
| `internal_user_viewer` | Read-only dashboard, keys, models, usage, and logs |

Blocking, deleting, or changing a user increments `auth_version`, invalidating
that user's existing UI sessions.

## Recommended rollout

1. Deploy the database migrations and application code while social auth is
   disabled.
2. Create local users with unique email addresses and the minimum required
   roles.
3. Register the OAuth applications and configure their exact callback URLs.
4. Enable social auth with `allow_verified_email_link: true` and temporarily
   keep `master_key_login_enabled: true` as break glass.
5. Enroll and verify the first `proxy_admin`.
6. Set both `master_key_login_enabled: false` and
   `allow_verified_email_link: false`.
7. Enroll subsequent users through their per-user **Sign-in Access** controls.
8. Confirm each operator can sign in and receives the intended permissions.

If every administrator is locked out, temporarily restore
`master_key_login_enabled: true` through the deployment configuration, sign in
with the master key, repair the local user or identity, and disable break glass
again.

## Security properties

- SCS cookies are `HttpOnly`, `SameSite=Lax`, and `Secure` by default.
- Production sessions are stored in PostgreSQL and have an 8-hour lifetime
  with a 30-minute idle timeout.
- Session tokens are renewed after both master-key and social login.
- OAuth callback state is compared in constant time and cannot be replayed.
- Provider errors and tokens are not returned to the browser.
- Unknown roles and unavailable, disabled, or version-mismatched users fail
  closed.
