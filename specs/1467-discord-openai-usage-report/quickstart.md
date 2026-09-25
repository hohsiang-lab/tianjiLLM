# Quickstart: Validate Discord OpenAI Credential Usage Report

## Preconditions

1. Work from `/Users/norman/src/github.com/hohsiang-lab/tianjiLLM`.
2. Use test-only `httptest` Discord endpoints and fake OpenAI usage snapshots;
   do not use a real credential or webhook URL in automated tests.
3. A production rollout must verify, without printing values:
   - database-backed credentials are configured;
   - the existing Discord webhook setting resolves to a non-empty value;
   - Redis is available when the Tianji deployment has more than one replica.

## Focused validation

Run the new tests first:

```sh
rtk go test ./internal/proxy/handler ./internal/callback ./internal/scheduler -run 'Test(OpenAISubscriptionDiscordUsageReport|DiscordUsageReportSender|OpenAISubscriptionDiscordUsageReportJob)' -count=1
```

Expected results:

- a report contains one safe row for each seeded OpenAI subscription credential;
- an empty seeded inventory produces one clearly labeled zero-credential report
  when the test destination is configured;
- fresh rows contain primary/weekly/reset/plan values and additional buckets
  when present;
- disabled, reconnect-required, stale, backoff, and unavailable rows remain
  safe and do not expose token-shaped fixture values, email, credential name,
  or organization data;
- a multi-part report preserves order and stays within Discord's content limit;
- a 50-row report reaches the responsive test destination within the
  60-second report context;
- webhook requests use `wait=true` and block mentions;
- only a confirmed rate limit receives one bounded retry, and a delivery
  failure does not alter a proxy request result;
- a second scheduler runner skips while the first holds the existing lock.

## Broader validation

```sh
rtk go test ./internal/proxy/handler ./internal/callback ./internal/scheduler -count=1
rtk make lint
rtk make test
rtk make build
```

Run only the narrowest applicable commands first. If a full quality gate is
not available locally, record the exact command and failure rather than
claiming it passed.

## Controlled runtime smoke check

After deployment to an approved non-production environment:

1. Configure a test Discord destination through the existing deployment secret
   path; do not add a report-specific setting.
2. Verify one report arrives after the configured 30-minute job interval or
   trigger the job through an approved operator/test harness.
3. Confirm its rows and part ordering match the UI's safe Codex usage states.
4. Verify the service logs contain only counts/statuses and no webhook URL,
   tokens, bearer/JWT values, raw upstream body, or raw Discord response.
5. In a multi-replica environment, confirm one runner holds the scheduler
   lock and only that runner starts delivery.

Do not treat a successful unit test or report receipt as proof of production
configuration correctness; recheck the actual replica count, Redis
connectivity, resolved webhook setting, and service logs at rollout time.
