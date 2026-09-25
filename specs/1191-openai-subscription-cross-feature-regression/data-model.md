# Data Model: Regression Matrix

This issue does not add runtime data models. It introduces a documentation/test-planning entity used during implementation and review.

## Entity: Regression Matrix Row

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `acceptance_path` | string | yes | Behavior or invariant from HO-1191 / HO-1173 / HO-1165. |
| `owner_issue` | string | yes | HO issue or group that originally owns the behavior. |
| `layer` | enum | yes | `unit`, `integration`, `ui_e2e`, `ci_guard`, or `multi_layer`. |
| `test_file` | path | yes | Current or planned test file. |
| `test_name` | string | yes | Current or planned Go test / E2E test name. |
| `command` | string | yes | Command that runs the row locally/CI. |
| `status` | enum | yes | `covered`, `partial`, `gap_fixed`, `blocked`. |
| `gap_action` | string | no | Required action when status is not `covered`. |
| `ci_evidence` | string | no | PR run/check evidence added after implementation. |

## Status Rules

- `covered`: Test exists and asserts the acceptance path directly.
- `partial`: Test exercises nearby code but misses a required assertion or layer.
- `gap_fixed`: This PR adds/updates tests to close the row.
- `blocked`: Only allowed when an external dependency prevents local execution; must include exact blocker and CI fallback.

## Required Matrix Groups

- Config and API-key compatibility.
- OAuth PKCE/connect/callback.
- Credential persistence and redaction.
- Refresh/rotation/lock/failover/routing.
- Quota and rate-limit state.
- Spend/audit attribution.
- Credential API lifecycle.
- Credentials UI and callback UI.
- Models page multi-credential selection.
- No-real-OpenAI CI guard.
