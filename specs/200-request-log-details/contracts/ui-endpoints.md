# UI Endpoint Contracts: Request Log Details

## GET /ui/logs/detail?request_id={id}

**Purpose**: Fetch log detail partial for the drawer (HTMX target)

**Request**: Query param `request_id` (string, required)

**Response**: HTML partial (templ rendered `LogDetailDrawer` component)

**Data flow**:
1. Query `GetSpendLogDetail` (SpendLogs LEFT JOIN ErrorLogs)
2. If `store_prompts_in_logs` enabled, query `GetRequestPayload`
3. Render `LogDetailDrawer` templ component with data
4. Return HTML partial for HTMX swap

**Success response** (200): HTML containing:
- Header: model name, status badge, request ID, timestamp
- Request Details card: model, provider, call_type, api_base
- Metrics card: tokens, cost, duration, cache status, start/end time
- Error alert (if failed): error code, type, message, traceback
- Request & Response section (if payload available) or guidance notice
- Metadata section (if non-empty)

**Error response** (404): Log not found message

## GET /ui/logs/table (existing — no change)

Existing endpoint. Returns the logs table partial. No contract changes needed.

## Interaction Flow

```
User clicks table row
  → HTMX hx-get="/ui/logs/detail?request_id=XXX"
  → hx-target="#log-detail-drawer"
  → Server renders LogDetailDrawer partial
  → HTMX swaps content into drawer container
  → Sheet opens (client-side JS via data attributes)

User clicks close / presses Escape
  → Sheet closes (client-side JS)
  → Drawer content cleared

User clicks different row
  → Same flow as above, HTMX replaces drawer content
```
