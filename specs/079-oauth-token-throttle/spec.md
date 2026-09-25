# Feature Specification: OAuth Token 智能限流

**Feature Branch**: `079-oauth-token-throttle`
**Created**: 2026-03-05
**Status**: Draft
**Input**: User description: "限制某一把 OAuth token，Anthropic OAuth Rate Limits 5h/7day 达到 80% 之后就不要再继续发送。多 token 场景下跳过高利用率 token + Discord 告警。所有 token 都达到阈值时返回 429。"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 自动跳过高利用率 Token (Priority: P1)

作为 LLM proxy 的运维人员，当系统配置了多把 Anthropic OAuth token 时，我希望系统自动跳过利用率达到阈值（默认 80%）的 token，将请求路由到利用率较低的 token，从而避免触发 Anthropic 的硬性限流（429）。

**Why this priority**: 这是核心功能。没有这个能力，系统会持续向即将被限流的 token 发送请求，导致用户请求失败。这直接影响服务可用性。

**Independent Test**: 配置 2 把以上 OAuth token，模拟其中 1 把利用率超过 80%，验证后续请求只路由到其他健康 token。

**Acceptance Scenarios**:

1. **Given** 配置了 3 把 OAuth token（A、B、C），Token A 的 5h 利用率为 85%, **When** 客户端发送请求, **Then** 系统跳过 Token A，从 B 和 C 中选择一把转发请求
2. **Given** Token B 的 7d 利用率为 90%（超过阈值）, **When** 客户端发送请求, **Then** 系统同样跳过 Token B
3. **Given** Token A 的 5h 利用率从 85% 降回到 60%, **When** 客户端发送请求, **Then** Token A 重新进入可用池，参与轮转选择
4. **Given** Token A 的状态为 "rate_limited"（已被 Anthropic 硬性限流）, **When** 客户端发送请求, **Then** 系统跳过该 token，无论其利用率数值

---

### User Story 2 - 所有 Token 耗尽时返回 429 (Priority: P1)

作为 LLM proxy 的用户，当所有可用的 OAuth token 都达到利用率阈值时，我希望系统返回清晰的 429 错误，并告知最近的 reset 时间，让我知道何时可以重试。

**Why this priority**: 与 P1 并列。当所有 token 都不可用时，必须有明确的降级行为，不能静默失败或返回模糊错误。

**Independent Test**: 配置所有 token 的利用率都超过阈值，发送请求，验证收到 429 状态码和 Retry-After header。

**Acceptance Scenarios**:

1. **Given** 所有配置的 OAuth token 利用率均超过阈值, **When** 客户端发送请求, **Then** 系统返回 HTTP 429，附带 Retry-After header 指示最近的 reset 时间
2. **Given** 所有 token 都超阈值，其中 Token A reset 时间最近（30 分钟后）, **When** 客户端收到 429, **Then** Retry-After 值约为 1800 秒
3. **Given** 系统返回 429 后, Token A 的利用率 reset 回到阈值以下, **When** 客户端重试, **Then** 请求成功路由到 Token A

---

### User Story 3 - Discord 告警通知 (Priority: P2)

作为运维人员，当某把 OAuth token 的利用率超过阈值或被 Anthropic 限流时，我希望收到 Discord 通知，以便及时了解系统状态并采取行动（如增加 token）。

**Why this priority**: 告警是运维的眼睛。虽然系统已经自动跳过高利用率 token，但运维需要知道这件事正在发生，以便评估是否需要扩容。

**Independent Test**: 配置 Discord webhook，模拟 token 利用率超过阈值，验证 Discord 收到告警消息。

**Acceptance Scenarios**:

1. **Given** 配置了 Discord webhook URL 和阈值 80%, **When** 某 token 的 5h 利用率首次超过 80%, **Then** 系统发送 Discord 告警，包含 token 标识、利用率百分比、reset 时间
2. **Given** 某 token 已触发告警, **When** 同一 token 在冷却期内（1 小时）再次超过阈值, **Then** 系统不重复发送告警（防止告警风暴）
3. **Given** 某 token 的 UnifiedStatus 变为 "rate_limited", **When** 系统解析到该状态, **Then** 系统发送 Discord 告警通知 token 已被 Anthropic 硬性限流
4. **Given** 未配置 Discord webhook URL, **When** token 超过阈值, **Then** 系统正常执行限流逻辑，不发送任何通知，不产生错误

---

### Edge Cases

- 首次请求时 store 中无任何 token 利用率数据：所有 token 视为可用，正常轮转
- Token 利用率数据缺失（-1 sentinel 值）：视为可用，不触发限流
- 混合配置 OAuth token 和普通 API key：普通 API key 永远不被限流逻辑影响
- 只配置了 1 把 OAuth token 且超过阈值：直接返回 429
- 多个 model config 使用同一把 OAuth token（重复 API key）：按 API key 去重，不重复检查

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统 MUST 在选择 upstream token 时，检查每把 OAuth token 的 5h 和 7d 利用率
- **FR-002**: 系统 MUST 跳过利用率（5h 或 7d 任一）达到可配置阈值（默认 80%）的 OAuth token
- **FR-003**: 系统 MUST 跳过 UnifiedStatus 为 "rate_limited" 的 OAuth token（已被 Anthropic 硬性限流）
- **FR-004**: 当所有 OAuth token 均超过阈值时，系统 MUST 返回 HTTP 429，附带 Retry-After header（值为最近的 reset 时间距当前的秒数）
- **FR-005**: 系统 MUST 在 token 利用率超过阈值时发送 Discord 告警（需配置 webhook URL）
- **FR-006**: Discord 告警 MUST 包含 token 标识（脱敏）、利用率百分比、限流窗口（5h/7d）、reset 时间
- **FR-007**: Discord 告警 MUST 有冷却机制（同一 token 同一类型告警在冷却期内不重复发送）
- **FR-008**: 非 OAuth token（普通 API key）MUST NOT 受限流逻辑影响
- **FR-009**: store 中无利用率记录的 token MUST 视为可用（不阻止首次请求）
- **FR-010**: 利用率数据缺失（sentinel 值 -1）MUST 视为可用
- **FR-011**: 阈值 MUST 可通过配置文件设置（`ratelimit_alert_threshold`），默认 0.8（80%）

### Key Entities

- **OAuth Token State**: 每把 OAuth token 的利用率状态，包含 token 标识（脱敏 hash）、5h 利用率、7d 利用率、统一状态（allowed/rate_limited/overage）、各窗口 reset 时间
- **Throttle Decision**: 每次请求的选择结果 — 选中的可用 token，或"全部耗尽"错误（附带最近 reset 时间）
- **Discord Alert**: 告警事件记录，包含 token 标识、告警类型（5h/7d/rate_limited）、冷却状态

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 当任一 token 利用率超过配置阈值时，该 token 被 100% 跳过，不接收任何新请求
- **SC-002**: 多 token 场景下，高利用率 token 被跳过后，剩余健康 token 均匀接收请求（轮转分配）
- **SC-003**: 所有 token 耗尽时，100% 的请求在 100ms 内返回 429 + Retry-After header（不等待上游超时）
- **SC-004**: token 利用率恢复到阈值以下后，在下一次请求即可重新参与轮转（无需手动干预）
- **SC-005**: Discord 告警在 token 超阈值后 5 秒内送达（排除网络延迟）
- **SC-006**: 同一 token 同一类型告警在 1 小时内最多发送 1 次
