# Spec: Reduce Double DB Query on Virtual Key Auth (HO-10)

## Problem
In `DBValidator`, `ValidateToken()` and `GetGuardrails()` each call `GetVerificationToken()` separately with the same `tokenHash`. This results in two identical DB queries per authenticated request.

## Solution
Merge into a single DB call by having `ValidateToken` also return guardrail names (policies). Remove the separate `GuardrailProvider` interface usage in `auth.go`.

## Changes
1. **`db_validator.go`**: Extend `ValidateToken` return to include `guardrails []string`. Remove `GetGuardrails` method.
2. **`auth.go`**: Update `TokenValidator` interface to return guardrails. Remove `GuardrailProvider` interface and the type-assertion block. Set guardrails from `ValidateToken` result directly.
3. **`db_validator_test.go`**: Update tests for new signature, remove `GetGuardrails` tests.
4. **`auth_test.go`**: Update mock to match new interface.

## Non-goals
- No SQL changes
- No caching
- No new dependencies
