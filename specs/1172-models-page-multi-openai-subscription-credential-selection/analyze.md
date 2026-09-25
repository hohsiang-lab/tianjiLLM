# Analyze: HO-1172 Models page multi OpenAI subscription credential selection

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0
- Governance blocker: implementation cannot start until Linear moves to `In Progress`

## Linear Scope Coverage

| Linear explicit behavior | Spec coverage | Plan/tasks coverage |
| --- | --- | --- |
| Models page supports selecting multiple `openai_subscription` credential IDs | FR-001 to FR-004, US1 | T004, T012-T023, T027 |
| Persist selected IDs into `model_list[].tianji_params.openai_subscription_credential_ids` | FR-004, data model | T027-T030 |
| Display selected credential metadata safely | FR-008 to FR-010, US5 | T014-T015, T019, T032-T034 |
| Prevent invalid combination with custom `api_base` in UI | FR-006, US3 | T006, T024, T028, T030 |
| Preserve existing `api_key` / `$OPENAI_API_KEY` UI/config behavior | FR-005, US4 | T007, T031, T035 |
| If no subscription credentials selected, existing api-key behavior remains unchanged | FR-005, US4 | T007, T027, T029 |
| UI E2E for multi-select add/remove/save | FR-013 | T004-T005 |
| UI blocks subscription IDs + custom `api_base` | FR-006, FR-013 | T006 |
| Existing api-key model creation/edit regression | FR-005, FR-013 | T007, T035 |
| Saved model config round-trips selected credential IDs | FR-004, SC-001, SC-002 | T004-T005 |
| Waiting Merge follow-up: use staging GitHub runner | FR-015, SC-007 | T044-T048 |

## Cross-Artifact Consistency

- `spec.md` requires exact `openai_subscription_credential_ids` persistence; `data-model.md` and `contracts/models-openai-subscription-ui.md` define the same JSON field。
- `plan.md` and `tasks.md` both require RED E2E before implementation。
- `research.md` confirms current Models UI lacks selector and Credentials UI already has safe metadata patterns。
- `quickstart.md` and `tasks.md` both keep production implementation blocked until Linear `In Progress`。
- The custom `api_base` conflict is represented in spec, contract, plan, tasks, and E2E acceptance。
- 2026-05-09 owner follow-up adds CI runner routing; `spec.md` FR-015 / SC-007 and `tasks.md` Phase 8 cover the workflow-only change。

## Out-of-Scope Challenge

- Credential lifecycle APIs and OpenAI account connect/test/refresh are excluded because sibling issues own those flows。
- Backend token resolution is excluded because HO-1167 already owns config parsing/resolution, and this issue is UI selection/persistence。
- Generic `api_key` credential selection is excluded because Linear explicitly says OpenAI subscription credential selection。
- Real OpenAI network calls are excluded because UI can validate with seeded DB rows and stored metadata。

## UI Mockscreen Scope Review

- Required states: Models table with selected credential badges, Add/Edit dialog multi-select list, invalid custom `api_base` conflict, empty credential list fallback。
- Mockscreen files are generated outside git under `/Users/n0rmanc/.openclaw/media/HO-1172-models-subscription-credentials-*`。
- Scope alignment: mockscreen focuses on compact operational Models page controls, not a landing/marketing layout; it reuses existing table/dialog mental model。

## E2E Coverage Map

- PR layer: TianjiLLM admin Models UI。
- Runtime boundary: browser -> HTMX Models handlers -> `ProxyModelTable.tianji_params` + `CredentialTable` safe metadata。
- Test command: `go test ./test/e2e -tags e2e -run 'TestModel(Create|Edit)_OpenAISubscription|TestModelOpenAISubscription|TestModelCreate_APIKeyRegression' -count=1`。
- CI evidence: full PR workflow after implementation。
- Gap/follow-up: no lifecycle coverage required; lifecycle remains HO-1185/sibling scope。

## Final Scope Confirmation

HO-1172 Todo planning is internally consistent and does not need an owner question. Move to `Waiting` after draft PR exists, docs-only diff passes, and mockscreen evidence is posted。

## Waiting Merge Follow-up Analysis - 2026-05-09

- Trigger: Norman requested PR #158 be changed to use the staging GitHub runner after HO-1172 had already reached `Waiting Merge`。
- State handling: because this changes PR scope after merge gate, Linear was moved back to `In Progress` before editing `.github/workflows/ci.yml`。
- Repo reality: TianjiLLM has a single CI workflow with `lint`、`test`、`e2e`、`build`、`docker` jobs, all previously using `ubuntu-latest`。
- Runner reality: existing staging ARC runner scale set is named `staging-monster-ci`; GitHub runner API showed online busy runners named `staging-monster-ci-...` at the time of change。
- GitHub docs check: `jobs.<job_id>.runs-on` supports a single string runner label; GitHub's self-hosted runner docs include ARC runner scale sets in the self-hosted runners list and advise copying the runner label into workflow YAML。
- Analysis result: fatal 0 / critical 0 / owner input 0 for replacing TianjiLLM CI job `runs-on` values with `staging-monster-ci` and letting PR CI prove pickup before returning to Waiting Merge。
