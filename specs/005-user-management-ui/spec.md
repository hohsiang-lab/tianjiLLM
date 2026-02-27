# Feature Specification: User Management UI

**Feature Branch**: `feat/user-management-ui`
**Created**: 2026-02-27
**Status**: Draft
**Linear**: HO-19

## User Scenarios & Testing

### User Story 1 - User List (Priority: P1)

An admin navigates to `/users` and sees all users with pagination. They can search by name/email and filter by role/status.

**Acceptance Scenarios**:
1. **Given** an admin visits `/users`, **When** the page loads, **Then** all users are listed with columns: Name, Email, Role, Status, Created, Last Active.
2. **Given** users exist, **When** admin types in the search box, **Then** results filter by name or email (HTMX partial update).
3. **Given** multiple pages of users, **When** admin clicks next page, **Then** pagination works via HTMX swap.

### User Story 2 - Create User (Priority: P1)

An admin clicks "Add User" on the list page, fills in the form, and creates a new user.

**Acceptance Scenarios**:
1. **Given** admin clicks "Add User", **When** dialog opens, **Then** form shows: name, email, role (dropdown), max_budget.
2. **Given** valid form data, **When** admin submits, **Then** user is created and list refreshes with new user.
3. **Given** duplicate email, **When** admin submits, **Then** error message shows.

### User Story 3 - User Detail Page (Priority: P1)

An admin clicks a user row and sees their detail page with full info, associated teams, keys, and spend.

**Acceptance Scenarios**:
1. **Given** admin clicks a user, **When** detail page loads, **Then** shows: basic info, teams, API keys, spend summary.
2. **Given** user has teams, **When** detail loads, **Then** team memberships are listed with team names.
3. **Given** user has spend, **When** detail loads, **Then** spend summary shows total and per-model breakdown.

### User Story 4 - Edit User (Priority: P2)

An admin edits a user's name, role, or budget from the detail page.

**Acceptance Scenarios**:
1. **Given** admin clicks "Edit" on detail page, **When** form opens, **Then** fields are pre-filled with current values.
2. **Given** admin changes role, **When** submits, **Then** role updates and detail page refreshes.

### User Story 5 - Disable/Enable User (Priority: P2)

An admin disables a user, blocking their API access. Re-enabling restores access.

**Acceptance Scenarios**:
1. **Given** active user, **When** admin clicks "Disable", **Then** user status changes to disabled.
2. **Given** disabled user, **When** admin clicks "Enable", **Then** user status changes to active.

### User Story 6 - Delete User (Priority: P3)

An admin deletes a user with confirmation dialog. This is a soft delete.

**Acceptance Scenarios**:
1. **Given** admin clicks "Delete", **When** confirmation dialog appears and admin confirms, **Then** user is soft-deleted.

### Edge Cases
- User with active API keys: show warning before disable/delete
- User who is the last admin: prevent role change/delete
- Empty user list: show "No users yet" state

## Requirements

### Functional Requirements
- **FR-001**: Admin can view paginated user list at `/users`
- **FR-002**: Admin can search users by name or email
- **FR-003**: Admin can filter users by role and status
- **FR-004**: Admin can create a new user with name, email, role, max_budget
- **FR-005**: Admin can view user detail page at `/users/{user_id}`
- **FR-006**: User detail shows associated teams, API keys, and spend summary
- **FR-007**: Admin can edit user name, role, max_budget
- **FR-008**: Admin can disable/enable a user
- **FR-009**: Admin can soft-delete a user (with confirmation)
- **FR-010**: Sidebar navigation includes "Users" link

### Key Entities
- **UserTable**: Existing DB table with user_id, user_alias, user_email, user_role, teams, spend, etc.
- **User Roles**: `proxy_admin` (admin), `internal_user` (member), `internal_user_viewer` (viewer)
- **Status**: Derived from metadata JSONB field (`metadata->>'status'` = 'active'|'disabled')

## Success Criteria
- **SC-001**: All CRUD operations work via HTMX without full page reloads
- **SC-002**: User list pagination handles 100+ users smoothly
- **SC-003**: E2E tests cover list, create, edit, disable, delete flows
- **SC-004**: Follows existing Teams/Orgs UI patterns (consistent UX)

## Assumptions
- Status stored in `metadata` JSONB (no schema migration needed)
- Soft delete = set `metadata->>'status'` to 'deleted' (not actual DELETE)
- Search is case-insensitive ILIKE on name + email
- No password management in v1 (users auth via API keys or SSO)
