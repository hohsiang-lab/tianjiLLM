# Feature Specification: Models Multi-Select for Create API Key

**Feature Branch**: `001-models-multiselect`
**Created**: 2026-02-24
**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Select Specific Models When Creating a Key (Priority: P1)

An administrator is creating a new API key and wants to restrict it to a specific subset of models (e.g., only allow a team to use certain models). Instead of typing model names manually in a text field, they open a dropdown list that shows all currently configured models, tick the ones they want, and save.

**Why this priority**: This is the core value of the feature — replacing error-prone free-text input with a structured, discoverable selection experience. Without this, the entire feature has no value.

**Independent Test**: Can be fully tested by opening the Create API Key form, interacting with the Models dropdown, selecting one or more models, submitting the form, and verifying the resulting key is restricted to only those models.

**Acceptance Scenarios**:

1. **Given** the administrator opens the Create API Key form, **When** they interact with the Models selector, **Then** they see a list of all model names currently configured in the proxy — no need to type or guess.
2. **Given** the administrator selects two specific models from the list and submits the form, **When** the key is created, **Then** the key's allowed model list contains exactly those two models.
3. **Given** an administrator selects a model, **When** they change their mind and deselect it before submitting, **Then** the model is removed from the selection and not included in the final key.
4. **Given** the administrator has already selected some models, **When** they view the form before submitting, **Then** they can clearly see which models are currently selected.

---

### User Story 2 - Create a Key with No Model Restriction (Priority: P1)

An administrator wants to create a general-purpose API key that is not restricted to any specific models — the key should work with all available models. They select the "All Models" option and submit the form.

**Why this priority**: Equally critical as story 1 because "All Models" is the default/unrestricted state. Administrators must be able to explicitly grant full model access without ambiguity.

**Independent Test**: Can be fully tested by selecting "All Models" in the Models selector, submitting the form, and confirming the resulting key has no model restrictions (works with any model).

**Acceptance Scenarios**:

1. **Given** the Create API Key form is open, **When** the administrator views the Models selector, **Then** an "All Models" option is visible and clearly labeled.
2. **Given** the administrator selects "All Models" and submits the form, **When** the key is created, **Then** the key has no model restrictions — it can be used with any model in the proxy.
3. **Given** "All Models" is selected, **When** the administrator also tries to select individual models, **Then** selecting "All Models" clears all individual model selections (or individual selections are ignored), resulting in an unrestricted key.
4. **Given** no specific model has been selected by the administrator, **When** the form is submitted, **Then** the key is treated as having no model restrictions (equivalent to "All Models").

---

### User Story 3 - Empty Model List in Proxy Config (Priority: P2)

An administrator is creating an API key but the proxy currently has no models configured in its model list. The form should still work gracefully.

**Why this priority**: Edge case that must not break the form. An administrator should still be able to create unrestricted keys even if no models are configured.

**Independent Test**: Can be fully tested by accessing the Create API Key form when the proxy has an empty model_list, verifying the form renders without errors, and verifying "All Models" is still selectable.

**Acceptance Scenarios**:

1. **Given** the proxy has no models configured, **When** the administrator opens the Create API Key form, **Then** the Models selector displays only the "All Models" option (no individual models).
2. **Given** the proxy has no models configured, **When** the administrator submits the form with "All Models" selected, **Then** the key is created successfully with no model restrictions.

---

### Edge Cases

- What happens when the proxy's model list changes after a key was created with specific models? (Key restrictions remain based on what was saved at creation time — no retroactive changes.)
- What happens if a previously selected model no longer exists in the proxy config? (The key retains the restriction; it's the caller's responsibility to use a valid model.)
- What happens when the administrator selects many models (e.g., 20+)? The selection must remain usable and all selected models must be preserved on submit.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Models field in the Create API Key form MUST present a selectable list of model names sourced from the proxy's currently configured model list.
- **FR-002**: The Models selector MUST include an "All Models" option as the first or clearly distinguished item.
- **FR-003**: Selecting "All Models" MUST result in an API key with no model restrictions (equivalent to an empty allowed-models list).
- **FR-004**: Selecting one or more specific models (without "All Models") MUST result in an API key whose allowed model list contains exactly the selected models.
- **FR-005**: The Models selector MUST support selecting multiple models simultaneously (multi-select behavior).
- **FR-006**: The current selection MUST be clearly visible to the administrator before they submit the form.
- **FR-007**: When the proxy's model list is empty, the Models selector MUST still render and allow the administrator to create an unrestricted ("All Models") key.
- **FR-008**: Selecting "All Models" MUST take precedence over any individual model selections, producing an unrestricted key.
- **FR-009**: The Models selector MUST replace the current free-text input field — the old comma-separated text entry MUST be removed.

### Key Entities

- **API Key**: Represents a credential granting access to the proxy. Has an `allowedModels` list — empty means unrestricted; non-empty means restricted to those model names.
- **Model**: A named LLM endpoint configured in the proxy. Identified by its model name string (e.g., `gpt-4`, `anthropic/claude-sonnet-4-5`).
- **Model List**: The set of all models currently configured in the proxy. Serves as the source of options for the Models selector.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An administrator can select models for a new API key without typing any text — 100% of model selection is done through the structured selector.
- **SC-002**: An administrator can identify and select the correct model in under 30 seconds when the proxy has up to 50 configured models.
- **SC-003**: The "All Models" option is visible and selectable on the first interaction with the form — no scrolling or searching required.
- **SC-004**: Zero API keys are created with incorrect model restrictions due to typos (improvement from the free-text input approach).
- **SC-005**: The Create API Key form continues to function correctly (no regressions) for all other fields when the Models selector is introduced.

## Assumptions

- The proxy's model list is accessible at the time the Create API Key form is rendered; it does not need to refresh dynamically after the form is opened.
- The number of configured models in the proxy is expected to be manageable (typically under 100) — no pagination is needed for the model list in the selector.
- Model names are unique within the proxy configuration.
- The "All Models" option is represented in the backend by an empty `allowedModels` array — no new data format is needed.
- Administrators who can access the Create API Key form already have sufficient permissions — no additional access control is required for the model list display.
