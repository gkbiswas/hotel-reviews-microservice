-- Migration Rollback: 003_auth_schema_rollback.sql
-- Description: Rollback authentication and authorization schema
-- Date: 2025-01-18

BEGIN;

-- Drop triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
DROP TRIGGER IF EXISTS update_permissions_updated_at ON permissions;
DROP TRIGGER IF EXISTS update_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_login_attempts_created_at;
DROP INDEX IF EXISTS idx_login_attempts_ip_address;
DROP INDEX IF EXISTS idx_login_attempts_email;

DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP INDEX IF EXISTS idx_audit_logs_resource;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_user_id;

DROP INDEX IF EXISTS idx_api_keys_deleted_at;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_api_keys_is_active;
DROP INDEX IF EXISTS idx_api_keys_key;
DROP INDEX IF EXISTS idx_api_keys_user_id;

DROP INDEX IF EXISTS idx_sessions_deleted_at;
DROP INDEX IF EXISTS idx_sessions_is_active;
DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_refresh_token;
DROP INDEX IF EXISTS idx_sessions_access_token;
DROP INDEX IF EXISTS idx_sessions_user_id;

DROP INDEX IF EXISTS idx_role_permissions_permission_id;
DROP INDEX IF EXISTS idx_role_permissions_role_id;

DROP INDEX IF EXISTS idx_user_roles_role_id;
DROP INDEX IF EXISTS idx_user_roles_user_id;

DROP INDEX IF EXISTS idx_permissions_deleted_at;
DROP INDEX IF EXISTS idx_permissions_is_active;
DROP INDEX IF EXISTS idx_permissions_resource_action;

DROP INDEX IF EXISTS idx_roles_deleted_at;
DROP INDEX IF EXISTS idx_roles_is_active;
DROP INDEX IF EXISTS idx_roles_name;

DROP INDEX IF EXISTS idx_users_deleted_at;
DROP INDEX IF EXISTS idx_users_is_active;
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;

-- Drop tables (in reverse order of creation due to foreign key constraints)
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

COMMIT;