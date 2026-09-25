# Requirements Checklist: Org detail OpenAI credential panel

## Scope

- [x] Spec only covers Org detail credential panel。
- [x] Spec excludes production code during Todo。
- [x] Spec excludes direct lifecycle buttons on Org detail。
- [x] Spec excludes schema migration and OpenAI live calls。

## Requirements Quality

- [x] Every functional requirement is testable。
- [x] Empty state is specified。
- [x] Org isolation is specified。
- [x] Generic credential exclusion is specified。
- [x] Existing credential detail link route is specified。
- [x] Secret DOM omission is specified。
- [x] Owner-approved mockscreen alignment is specified as implementation acceptance criteria。

## Test Coverage

- [x] Populated panel E2E specified。
- [x] Empty state E2E specified。
- [x] Detail link navigation E2E specified。
- [x] Other-org exclusion E2E specified。
- [x] Generic credential exclusion E2E specified。
- [x] Secret material negative assertion specified。
- [x] Desktop/mobile result-screen comparison against approved mockscreen specified。

## Security

- [x] `credential_value` forbidden from rendering。
- [x] Raw token fields forbidden from rendering。
- [x] Arbitrary metadata map rendering forbidden。
- [x] Unknown quota/status fallback specified。

## Owner Input

- [x] Owner input required: 0。
