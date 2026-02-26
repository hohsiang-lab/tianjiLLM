# Feature Specification: Team & Organization Management UI

**Feature Branch**: `001-team-org-admin-ui`
**Created**: 2026-02-26
**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse and Manage Teams (Priority: P1)

As a proxy administrator, I want a dedicated Teams list page where I can see all teams at a glance, create new teams, and perform quick actions (block/unblock, delete) without navigating away from the list.

**Why this priority**: Teams are the primary unit for grouping API keys and enforcing budget/model access policies. Administrators need full CRUD visibility first — this is the foundation for everything else.

**Independent Test**: Navigate to `/ui/teams`, verify the list loads with team names, status badges, and action buttons. Create a new team, confirm it appears in the list. Block a team, confirm its status badge changes to "Blocked".

**Acceptance Scenarios**:

1. **Given** there are existing teams in the system, **When** the admin navigates to `/ui/teams`, **Then** a paginated table shows each team's alias, organization, member count, spend vs. budget, model count, blocked status, and creation date.
2. **Given** the admin is on the Teams list, **When** they click "New Team" and submit valid details (alias, optional budget, optional models), **Then** the new team appears at the top of the list without a full page reload.
3. **Given** a team is currently active, **When** the admin clicks "Block", **Then** the team's status changes to Blocked and the button label switches to "Unblock".
4. **Given** the admin clicks "Delete" on a team, **When** they confirm the dialog, **Then** the team is removed from the list.

---

### User Story 2 - View Team Details and Manage Members/Models (Priority: P2)

As a proxy administrator, I want a Team Detail page where I can inspect a team's complete configuration — members, allowed models, spend/budget breakdown, and metadata — and make targeted edits.

**Why this priority**: The list page provides overview; the detail page enables precise management. Without it, admins cannot add/remove individual members or models.

**Independent Test**: Click any team from the list, land on `/ui/teams/{team_id}`. The page shows member list, model list, spend vs. max_budget, RPM/TPM limits, and a metadata panel. Add a member, confirm they appear in the list.

**Acceptance Scenarios**:

1. **Given** a team with members and model restrictions, **When** the admin opens its detail page, **Then** they see: team alias, organization link, admin list, member list (with roles), allowed models, current spend, max budget, TPM/RPM limits, budget duration, blocked status, and metadata JSON.
2. **Given** the detail page is open, **When** the admin types a user ID and clicks "Add Member", **Then** the member appears in the list without full reload.
3. **Given** a member exists in the list, **When** the admin clicks the remove icon, **Then** the member is removed from the team immediately.
4. **Given** the admin wants to restrict model access, **When** they add or remove a model from the allowed models list, **Then** the change persists and the model list updates in place.
5. **Given** the admin edits the team alias or budget limits, **When** they submit the edit form, **Then** the page reflects the updated values.

---

### User Story 3 - Browse and Manage Organizations (Priority: P3)

As a proxy administrator, I want an Organizations list page where I can see all organizations, create new ones, edit them, and delete them.

**Why this priority**: Organizations are containers for teams; they require management but are typically fewer in number and changed less frequently than teams.

**Independent Test**: Navigate to `/ui/orgs`. Create a new organization, confirm it appears. Edit its alias and budget, confirm changes. Delete it.

**Acceptance Scenarios**:

1. **Given** organizations exist in the system, **When** the admin navigates to `/ui/orgs`, **Then** a table shows each organization's alias, team count, member count, spend vs. budget, and model restrictions.
2. **Given** the admin clicks "New Organization" and provides an alias and optional budget/models, **When** they submit, **Then** the organization appears in the list without full page reload.
3. **Given** an organization entry in the list, **When** the admin clicks "Edit" and changes the alias or max budget, **Then** the updated values are reflected immediately.
4. **Given** an organization has no dependent teams, **When** the admin clicks "Delete" and confirms, **Then** the organization is removed from the list.

---

### User Story 4 - View Organization Details and Manage Membership (Priority: P4)

As a proxy administrator, I want an Organization Detail page where I can see all members of an organization, add or remove members, and change member roles.

**Why this priority**: Organization membership controls which users belong to an org and their roles. This is needed for access governance but depends on the Org list being functional first.

**Independent Test**: Open `/ui/orgs/{org_id}`. Verify the member list displays user IDs, roles, spend, and join dates. Add a member with a role, confirm they appear. Change a role, confirm the update.

**Acceptance Scenarios**:

1. **Given** an organization with members, **When** the admin opens its detail page, **Then** they see org alias, max budget, current spend, allowed models, TPM/RPM limits, and a members table with columns: user ID, role, spend, budget ID, and join date.
2. **Given** the detail page is open, **When** the admin provides a user ID and role then clicks "Add Member", **Then** the member appears in the table without full reload.
3. **Given** a member exists in the org, **When** the admin changes their role via a dropdown and confirms, **Then** the role updates in the table.
4. **Given** an org member in the table, **When** the admin clicks "Remove" and confirms, **Then** the member is removed from the organization.

---

### Edge Cases

- What happens when an admin tries to delete an organization that still has teams linked to it? The admin may submit the delete request; the server detects dependent teams and returns a clear error. The UI displays this error inline. The delete button is NOT pre-emptively disabled — no extra count query is issued at render time.
- What happens when a user ID provided for team/org membership does not exist in the UserTable? The system should display a validation error rather than silently failing.
- What happens when the admin sets a budget that is less than the team's current spend? The system should warn but allow the update (no retroactive block).
- How does the system handle a blocked team receiving new API requests? The block/unblock operation is a data flag; actual enforcement is handled by the proxy middleware — the UI only needs to toggle the flag.
- What if the models list is empty for a team? An empty models list means the team inherits models from its parent organization or the global config — the UI should display "Inherited / All models" instead of an empty state.
- What happens when the same user is added as a member twice? The system should return a clear duplicate-member error and not corrupt the members array.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST display a paginated list of all teams on the Teams list page using server-side offset pagination with explicit page controls (not infinite scroll), including alias, organization, member count, spend, max budget, model count, blocked status, and creation date.
- **FR-002**: System MUST allow administrators to create a new team by providing at minimum a team alias; budget, models, TPM/RPM limits, and organization ID are optional at creation.
- **FR-003**: System MUST allow administrators to edit an existing team's alias, max budget, TPM/RPM limits, and budget duration from the detail page via a modal dialog.
- **FR-004**: System MUST allow administrators to block or unblock a team via a toggle action on either the list page or the detail page.
- **FR-005**: System MUST allow administrators to permanently delete a team with an explicit confirmation step.
- **FR-006**: System MUST display a Team Detail page at a stable URL containing the team's complete configuration: alias, organization, admins, members with roles, allowed models, spend vs. budget, TPM/RPM limits, budget duration, blocked status, budget reset date, and metadata.
- **FR-007**: System MUST allow administrators to add a user (by user ID) to a team's member list from the detail page.
- **FR-008**: System MUST allow administrators to remove a member from a team from the detail page.
- **FR-009**: System MUST allow administrators to add or remove models from a team's allowed models list from the detail page.
- **FR-010**: System MUST display a paginated list of all organizations on the Organizations list page using server-side offset pagination with explicit page controls (not infinite scroll), including alias, spend, max budget, model count, and creation date.
- **FR-011**: System MUST allow administrators to create a new organization with at minimum an alias; budget and models are optional.
- **FR-012**: System MUST allow administrators to edit an existing organization's alias and max budget via a modal dialog.
- **FR-013**: System MUST allow administrators to delete an organization with an explicit confirmation step.
- **FR-014**: System MUST display an Organization Detail page containing: alias, spend vs. max budget, allowed models, TPM/RPM limits, and a members table showing user ID, role, spend, and join date.
- **FR-015**: System MUST allow administrators to add an org member by providing user ID and role from the organization detail page.
- **FR-016**: System MUST allow administrators to change an existing org member's role.
- **FR-017**: System MUST allow administrators to remove a member from an organization with confirmation.
- **FR-018**: System MUST provide inline feedback (success/error messages) for all create, update, delete, and toggle operations without requiring a full page reload.
- **FR-019**: All list and detail pages MUST be accessible only to authenticated administrators.

### Key Entities

- **Team**: A named group with optional organization affiliation, an admin list, a member list with roles stored separately (valid team member roles: `"admin"` or `"member"`), allowed model restrictions, spend tracking, budget limits (max_budget, tpm_limit, rpm_limit, budget_duration), a blocked flag, and arbitrary metadata.
- **Organization**: A top-level container with an alias, allowed model restrictions, spend tracking, budget limits, and metadata. Organizations group teams and members but do not have a blocked flag.
- **Organization Membership**: A join record linking a user to an organization with an assigned role and optional per-member budget. A user may belong to at most one membership record per organization.
- **User**: Referenced by user ID in team members and org membership. Users are managed separately; this feature only reads user IDs for display and references them during add-member operations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An administrator can create a new team and see it appear in the list within 3 seconds of form submission.
- **SC-002**: An administrator can block or unblock a team with a single click, and the status change is visible in under 2 seconds without a page reload.
- **SC-003**: An administrator can add or remove a team member from the detail page and see the updated member list within 3 seconds.
- **SC-004**: An administrator can create, edit, and delete an organization in under 2 minutes total for all three operations combined.
- **SC-005**: An administrator can view a team's full detail page — including members, models, and spend data — within 3 seconds of navigating to its URL.
- **SC-006**: All destructive operations (delete team, delete org, remove member) require explicit confirmation, preventing accidental data loss.
- **SC-007**: All error states (duplicate member, non-existent user, org with dependent teams) display a human-readable message that identifies the problem without exposing internal system details.
- **SC-008**: The Teams and Organizations list pages remain responsive and usable with at least 200 entries without infinite loading states.

## Assumptions

- `proxy_admin` users have global access to all teams and organizations. `org_admin` users have scoped access limited to their own organization and its teams only; they cannot view or manage other organizations. No new role system is introduced.
- `organization_id` for a new team is optional; teams without an org are valid and displayed with a "No Organization" label.
- The `members_with_roles` JSONB column stores structured role assignments (e.g., `[{"user_id": "u1", "role": "member"}]`); the UI reads this for per-member role display.
- Deleting a team does not cascade to API keys that reference it; that is out of scope for this feature.
- The spend figures displayed are read-only aggregates from the `spend` column already maintained by the proxy middleware.
- Role values for organization membership follow the existing user role conventions in the system (e.g., `internal_user`, `proxy_admin`, `org_admin`).
- Budget duration values (e.g., `daily`, `weekly`, `monthly`) are displayed as-is; the UI does not validate or transform them.
- The metadata field is displayed and editable as a raw JSON text area; no schema enforcement is applied in the UI layer.

## Clarifications

### Session 2026-02-26

- Q: What is the access scope for `org_admin` vs `proxy_admin` on Teams and Organizations pages? → A: `proxy_admin` has global access; `org_admin` sees only their own organization and its teams (scoped access).
- Q: What pagination strategy should list pages use? → A: Server-side offset pagination with explicit page controls (not infinite scroll).
- Q: What role values are valid for team members (distinct from organization membership roles)? → A: Team member roles are a simplified set: `"admin"` or `"member"`.
- Q: When deleting an organization with dependent teams, should the UI pre-emptively disable the delete button or show an error after submission? → A: Submit → server detects dependent teams → display inline error (no pre-emptive UI disable, no extra count query at render time).
- Q: Should edit forms for team and organization configuration use inline editing or modal dialogs? → A: Modal-based edit forms for all team and organization configuration changes.
