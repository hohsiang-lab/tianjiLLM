-- 010_org_membership.sql
-- Organization membership table — tracks user roles within organizations.

CREATE TABLE IF NOT EXISTS "OrganizationMembership" (
    user_id         TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    user_role       TEXT,
    spend           DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    budget_id       TEXT REFERENCES "BudgetTable"(budget_id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, organization_id),
    UNIQUE (user_id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_org_membership_org ON "OrganizationMembership" (organization_id);
CREATE INDEX IF NOT EXISTS idx_org_membership_user ON "OrganizationMembership" (user_id);
