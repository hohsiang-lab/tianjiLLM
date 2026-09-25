# Requirements Checklist

## Completeness

- [x] Spec covers config/model parameter.
- [x] Spec covers Models UI selector.
- [x] Spec covers create and edit flows.
- [x] Spec covers invalid transport combinations.
- [x] Spec preserves API-key `openai/*` behavior.
- [x] Spec excludes backend transport implementation.
- [x] Spec excludes response/error normalization.

## Testability

- [x] Config validation tests are identified.
- [x] UI handler tests are identified.
- [x] E2E create/edit tests are identified.
- [x] API-key regression test is identified.
- [x] No real OpenAI/ChatGPT calls are required.

## Security

- [x] Token material is forbidden in UI DOM.
- [x] Credential display remains safe metadata only.
- [x] No credential lifecycle changes are introduced.

## Scope Confirmation

- [x] Fatal issues: 0.
- [x] Critical issues: 0.
- [x] Owner input required before implementation planning completion: 0.
