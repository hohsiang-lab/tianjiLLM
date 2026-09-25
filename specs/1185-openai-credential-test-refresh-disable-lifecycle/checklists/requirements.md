# Requirements Checklist: HO-1185

## Spec Quality

- [x] Scope is limited to test/refresh/disable lifecycle APIs。
- [x] Out-of-scope explicitly excluded production implementation during Todo; Linear is now In Progress。
- [x] User stories have independent tests。
- [x] Functional requirements are measurable。
- [x] Secret egress boundaries are explicit。
- [x] External OpenAI behavior is backed by official docs。
- [x] Dependencies on HO-1176/1177/1180/1182/1183/1184/1190 are listed。

## Safety

- [x] No real OpenAI credentials are required。
- [x] Tests must use mock upstream and no-real-OpenAI guard。
- [x] Responses must not expose token material。
- [x] Audit payloads must be redacted。
- [x] Disable is local-only and does not imply remote revoke。

## Gate

- [x] Todo output is docs-only。
- [x] No owner scope questions remain。
- [x] Linear moved to In Progress before production implementation started。
