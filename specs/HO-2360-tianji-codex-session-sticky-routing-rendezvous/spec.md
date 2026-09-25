# Feature Specification: Codex session sticky rendezvous routing

**Feature Branch**: `HO-2360-tianji-codex-session-sticky-routing-rendezvous`
**Created**: 2026-07-09
**Status**: Draft
**Input**: Linear HO-2360: Tianji Codex session sticky routing 改為 rendezvous 分流

## User Scenarios & Testing

### User Story 1 - 同一 Codex session 穩定選到同一 credential (Priority: P1)

Codex backend 使用 OpenAI subscription credentials 時，不同請求只要帶有同一個 `session_id`，就應該在該 credential 仍可選時穩定使用同一個 upstream credential，避免同一工作階段在多個帳號間跳動。

**Why this priority**: 這是修復 session sticky bug 的核心；沒有 session-level route key，後續分流或 fallback 都沒有正確基礎。

**Independent Test**: 使用多個可選 credential 與同一個 `session_id` 重複排序 candidate，第一順位 credential 必須一致；當同一 credential 仍低於 usage gate 時必須 reuse。

**Acceptance Scenarios**:

1. **Given** 一個 Codex request 帶有 `client_metadata.session_id` 且有多個可選 subscription credentials，**When** 多次排序 candidates，**Then** 第一順位 credential 必須一致。
2. **Given** 一個 Codex request 只在 `client_metadata["x-codex-turn-metadata"]` JSON string 內帶 `session_id`，**When** 進入 Codex backend ordering，**Then** 系統必須使用該 nested session id 形成 session sticky track。
3. **Given** sticky credential usage metadata 仍 selectable，**When** 同 session 新請求進來，**Then** 系統必須 reuse 原 sticky credential，而不是重新依最低分數或 round-robin 改選。

---

### User Story 2 - 不同 Codex sessions 以 rendezvous hashing 分散到可選 credentials (Priority: P2)

多個 Codex sessions 在同一 org / route class 下必須分散到目前可選 credentials，避免 org-level sticky 造成 hot credential。

**Why this priority**: 解決實際 hotspot：live logs 顯示多個 session 仍集中到少數 upstream token；rendezvous hashing 是已選定的分流策略。

**Independent Test**: 使用固定 session set 與 7 個可選 credentials 模擬分配，結果應跨多個 credentials；移除一個 credential 時，只有原本映射到該 credential 的 sessions 需要 remap。

**Acceptance Scenarios**:

1. **Given** 多個不同 `session_id` 與多個可選 credentials，**When** 建立新的 session sticky selection，**Then** sessions 必須分布到多個 credentials。
2. **Given** 某個 credential 從可選集合移除，**When** 重新排序同一批 sessions，**Then** 只有原本選到該 credential 的 sessions 需要換到剩餘可選 credentials。
3. **Given** candidate pool 已經排除 disabled、rate-limited、auth-failed、usage over gate credentials，**When** 執行 rendezvous selection，**Then** 不得選到不可用 credential。

---

### User Story 3 - 缺少 session id 時保留既有 fallback 且可觀測 (Priority: P3)

如果 Codex request 沒有可用 `session_id` 或 nested metadata JSON malformed，系統必須保留現有 org / route-class sticky 行為，並留下不洩漏 payload 的 safe observability。

**Why this priority**: live evidence 顯示 session id 通常存在，但 fallback 是相容性與診斷必要條件。

**Independent Test**: 對 missing、blank、malformed nested metadata 的請求排序，確認 route key 回到既有 org-level track，且 source 狀態可供 log/metric 使用。

**Acceptance Scenarios**:

1. **Given** `client_metadata` 沒有 session id，**When** Codex candidate ordering 執行，**Then** route key 必須維持既有 org / route-class sticky key。
2. **Given** nested `x-codex-turn-metadata` 不是合法 JSON，**When** extractor 執行，**Then** source 必須是 safe parse error 狀態，且不得使用 `thread_id`、`turn_id` 或 `window_id` 當 sticky key。
3. **Given** image generation / image edit request 沒有 session metadata contract，**When** 進入 Codex image path，**Then** 行為必須維持 org-level routing，不得擴 schema。

### Edge Cases

- `session_id` 是空字串或全空白時，必須視為 missing。
- direct `client_metadata.session_id` 與 nested `x-codex-turn-metadata.session_id` 同時存在時，direct 值優先。
- nested JSON malformed 時不得 panic，也不得把 raw metadata payload 寫入 log。
- credential over usage gate、exhausted、rate-limited、disabled、auth-failed 時，既有 `CanReuse` / re-evaluation 行為必須讓受影響 session 重新選擇。
- 非 Codex OpenAI subscription routing、native Anthropic upstream sticky、image generation/edit schema 不得因本 issue 改變。

## Requirements

### Functional Requirements

- **FR-001**: System MUST extract Codex session identity from request metadata with priority `client_metadata.session_id`, then `client_metadata["x-codex-turn-metadata"].session_id`.
- **FR-002**: System MUST trim extracted session id and treat empty result as missing.
- **FR-003**: System MUST NOT use `thread_id`, `turn_id`, or `window_id` as sticky routing keys.
- **FR-004**: System MUST use session-aware Codex sticky track `openai-subscription:<org>:codex-session:<session_id>:<route_class>` when session id exists.
- **FR-005**: System MUST preserve existing `openai-subscription:<org>:<route_class>` track when session id is missing or invalid.
- **FR-006**: System MUST select a new session sticky credential with rendezvous hashing over currently selectable credentials.
- **FR-007**: System MUST keep existing sticky reuse and usage-limit re-evaluation semantics for selectable, over-gate, exhausted, rate-limited, disabled, auth-failed, and primary reset conditions.
- **FR-008**: System MUST thread session identity into Codex candidate ordering for `/v1/responses` HTTP, `/v1/responses` WebSocket create frame, `/v1/responses/compact`, chat completion, and chat streaming.
- **FR-009**: System MUST leave image generation and image edit routing/schema unchanged for this issue.
- **FR-010**: System MUST emit safe logs/metrics for missing session id fallback and routing selection without raw bearer token, credential value, request body, full metadata payload, or unredacted session payload leakage.
- **FR-011**: System MUST NOT change non-Codex OpenAI subscription routing, native Anthropic upstream sticky behavior, database schema, migrations, or config/UI settings.

### Key Entities

- **Codex Session Identity**: Normalized session id and source (`direct`, `x-codex-turn-metadata`, `missing`, `parse_error`) derived from request metadata.
- **Codex Sticky Track**: In-memory sticky key used by routing strategy to remember selected credential per org / session / route class.
- **Selectable Credential Pool**: Subscription credentials after existing resolution, rate-limit, auth, disabled, and Codex usage gate filtering.
- **Rendezvous Score**: Deterministic score from `session_id + "\x00" + credential_id`; highest score wins for new sticky selection.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Same Codex session selects the same first credential in 100% of repeated routing attempts while that credential remains selectable.
- **SC-002**: A deterministic test set of at least 100 sessions maps to more than one credential when at least two credentials are selectable.
- **SC-003**: Removing one credential remaps only sessions that previously selected that credential in the deterministic rendezvous test.
- **SC-004**: Missing or malformed session metadata falls back to the existing org-level sticky key in 100% of fallback tests.
- **SC-005**: Required Go tests for `internal/proxy/handler`, `internal/provider/chatgptcodex`, and `internal/config` pass after implementation.
- **SC-006**: UI behavior remains unchanged; no new UI setting, toggle, page, DB table, migration, or image schema expansion appears in the final diff.
