# Requirements Checklist: HO-1300

## Scope Completeness

- [x] Linear problem statement is represented.
- [x] Root cause is represented.
- [x] Expected fix is represented.
- [x] Client `store:false` compatibility is represented.
- [x] Client `store:true` rejection is represented.
- [x] Normal OpenAI API-key no-regression is represented.
- [x] Secret-safety requirement is represented.

## Testability

- [x] Payload unit test can prove `store:false` is serialized.
- [x] Handler test can prove client `store:false` reaches mocked backend.
- [x] Handler test can prove `store:true` rejects before upstream.
- [x] OpenAI provider test can prove normal `ExtraParams` pass-through remains unchanged.
- [x] Tests use local mocks only.

## Governance

- [x] Todo phase contains no production code change.
- [x] Implementation is blocked until Linear `In Progress`.
- [x] Draft PR is docs-only.
- [x] Fatal issues: 0.
- [x] Critical issues: 0.
- [x] Owner input required: 0.
