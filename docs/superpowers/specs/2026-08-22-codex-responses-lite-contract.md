# Codex Responses Lite Contract

**Goal:** Make Tianji's ChatGPT Codex Responses and compact payloads match the
upstream Codex Responses Lite wire contract without changing non-Lite requests.

## Contract

When the selected Codex model uses Responses Lite:

- `input` is an array whose first item is `type=additional_tools`,
  `role=developer`, and contains the normalized tools.
- A non-empty top-level `instructions` value becomes a developer `message`
  input item.
- Top-level `instructions` and `tools` are omitted from the wire payload.
- Flat `function` and `custom` tools are grouped into one `functions` namespace;
  existing `functions` entries are merged while other namespaces keep order.
- Hosted Responses tools are omitted rather than sent to the Lite backend.
- `tool_choice` is `"auto"`, `parallel_tool_calls` is `false`, and
  `reasoning.context` is `"all_turns"`.
- Normal Responses keeps `store=false` and `stream=true`; compact removes
  `store` and `stream`.
- The prepared normalized body is reused byte-for-byte across credential
  retries.

The Lite header is added by the transport selected for the request. This
change does not alter that routing decision or non-Lite payload behavior.

## Scope

Change only the shared prepared-payload normalization path and its tests.
Image-generation hosted tools are not silently rewritten into client tools;
those requests remain an explicit follow-up compatibility decision.
