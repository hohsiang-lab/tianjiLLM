# Quickstart: Composite Score V2

## What changed

The OAuth token selection score now considers the **7d Sonnet** utilization window alongside 5h and 7d All Models. The formula changed from a quadratic weighted sum to a simple max-normalized score.

## How to verify

```bash
# Run all affected tests
go test ./internal/callback/... ./internal/proxy/handler/... ./internal/ui/... -v

# Check specific composite score tests
go test ./internal/proxy/handler/... -run "TestCompositeScore" -v
```

## Configuration

No configuration changes needed. The existing `ratelimit_alert_threshold` (default 0.80) is now also used for 5h normalization in the score formula.

## UI changes

The "Selection score" value on the Claude Code tab now ranges from 0 to ~1.0 (was 0 to 3.0). Color thresholds updated:
- Green: score < 0.5
- Orange: score < 0.8
- Red: score >= 0.8
