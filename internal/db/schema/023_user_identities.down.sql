DROP TABLE IF EXISTS "UserIdentityTable";

ALTER TABLE "UserTable"
    DROP COLUMN IF EXISTS auth_version;
