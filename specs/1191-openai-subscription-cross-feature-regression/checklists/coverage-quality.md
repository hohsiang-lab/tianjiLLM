# Coverage Quality Checklist

- [x] Every HO-1191 / HO-1173 acceptance path has a matrix row.
- [x] Every row names a test file, test name, and runnable command.
- [x] `covered` rows assert the acceptance path directly, not only adjacent behavior.
- [x] `partial` rows have an implementation task before Waiting Merge.
- [x] OAuth/token/upstream rows use local mocks or guarded clients.
- [x] UI rows assert persisted state or safe DOM where relevant.
- [x] Redaction rows check actual response/DOM/audit surfaces.
- [x] Existing `api_key` path remains explicitly covered.
- [x] Custom `api_base` + subscription ID validation remains explicitly covered.
- [ ] Final PR evidence includes CI lint/test/e2e/build status.
