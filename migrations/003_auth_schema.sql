-- Migration: 003_auth_schema.sql
-- Description: Add authentication and authorization schema
-- Date: 2025-01-18

BEGIN;

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    is_verified BOOLEAN DEFAULT FALSE,
    last_login_at TIMESTAMP,
    password_expiry TIMESTAMP,
    failed_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    two_factor_enabled BOOLEAN DEFAULT FALSE,
    two_factor_secret VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create roles table
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(20) NOT NULL CHECK (action IN ('create', 'read', 'update', 'delete', 'execute')),
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create user_roles junction table
CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id)
);

-- Create role_permissions junction table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (role_id, permission_id)
);

-- Create sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    user_agent TEXT,
    ip_address VARCHAR(45),
    device_id VARCHAR(255),
    last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create api_keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    key VARCHAR(255) NOT NULL UNIQUE,
    key_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    usage_count INTEGER DEFAULT 0,
    rate_limit INTEGER DEFAULT 1000,
    scopes JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create audit_logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    resource_id UUID,
    old_values JSONB,
    new_values JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    result VARCHAR(20) NOT NULL CHECK (result IN ('success', 'failure')),
    error_msg TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create login_attempts table
CREATE TABLE IF NOT EXISTS login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT,
    success BOOLEAN DEFAULT FALSE,
    failure_reason VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_roles_is_active ON roles(is_active);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles(deleted_at);

CREATE INDEX IF NOT EXISTS idx_permissions_resource_action ON permissions(resource, action);
CREATE INDEX IF NOT EXISTS idx_permissions_is_active ON permissions(is_active);
CREATE INDEX IF NOT EXISTS idx_permissions_deleted_at ON permissions(deleted_at);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_access_token ON sessions(access_token);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions(refresh_token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_is_active ON sessions(is_active);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted_at ON sessions(deleted_at);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key ON api_keys(key);
CREATE INDEX IF NOT EXISTS idx_api_keys_is_active ON api_keys(is_active);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_deleted_at ON api_keys(deleted_at);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

CREATE INDEX IF NOT EXISTS idx_login_attempts_email ON login_attempts(email);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_address ON login_attempts(ip_address);
CREATE INDEX IF NOT EXISTS idx_login_attempts_created_at ON login_attempts(created_at);

-- Insert default roles
INSERT INTO roles (name, description) VALUES 
    ('admin', 'Administrator with full system access'),
    ('user', 'Regular user with limited access'),
    ('moderator', 'Moderator with content management privileges'),
    ('service', 'Service account for API access')
ON CONFLICT (name) DO NOTHING;

-- Insert default permissions
INSERT INTO permissions (name, resource, action, description) VALUES
    -- User management permissions
    ('users.create', 'users', 'create', 'Create new users'),
    ('users.read', 'users', 'read', 'View user information'),
    ('users.update', 'users', 'update', 'Update user information'),
    ('users.delete', 'users', 'delete', 'Delete users'),
    
    -- Role management permissions
    ('roles.create', 'roles', 'create', 'Create new roles'),
    ('roles.read', 'roles', 'read', 'View role information'),
    ('roles.update', 'roles', 'update', 'Update role information'),
    ('roles.delete', 'roles', 'delete', 'Delete roles'),
    
    -- Permission management permissions
    ('permissions.create', 'permissions', 'create', 'Create new permissions'),
    ('permissions.read', 'permissions', 'read', 'View permission information'),
    ('permissions.update', 'permissions', 'update', 'Update permission information'),
    ('permissions.delete', 'permissions', 'delete', 'Delete permissions'),
    
    -- Review management permissions
    ('reviews.create', 'reviews', 'create', 'Create new reviews'),
    ('reviews.read', 'reviews', 'read', 'View reviews'),
    ('reviews.update', 'reviews', 'update', 'Update reviews'),
    ('reviews.delete', 'reviews', 'delete', 'Delete reviews'),
    
    -- Hotel management permissions
    ('hotels.create', 'hotels', 'create', 'Create new hotels'),
    ('hotels.read', 'hotels', 'read', 'View hotel information'),
    ('hotels.update', 'hotels', 'update', 'Update hotel information'),
    ('hotels.delete', 'hotels', 'delete', 'Delete hotels'),
    
    -- Provider management permissions
    ('providers.create', 'providers', 'create', 'Create new providers'),
    ('providers.read', 'providers', 'read', 'View provider information'),
    ('providers.update', 'providers', 'update', 'Update provider information'),
    ('providers.delete', 'providers', 'delete', 'Delete providers'),
    
    -- File processing permissions
    ('files.process', 'files', 'execute', 'Process uploaded files'),
    ('files.read', 'files', 'read', 'View file processing status'),
    
    -- API key management permissions
    ('api_keys.create', 'api_keys', 'create', 'Create new API keys'),
    ('api_keys.read', 'api_keys', 'read', 'View API key information'),
    ('api_keys.update', 'api_keys', 'update', 'Update API key information'),
    ('api_keys.delete', 'api_keys', 'delete', 'Delete API keys'),
    
    -- Audit log permissions
    ('audit_logs.read', 'audit_logs', 'read', 'View audit logs'),
    
    -- System administration permissions
    ('system.admin', 'system', 'execute', 'System administration privileges')
ON CONFLICT (name) DO NOTHING;

-- Assign permissions to roles
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'user' AND p.name IN (
    'reviews.read',
    'hotels.read',
    'providers.read',
    'api_keys.create',
    'api_keys.read',
    'api_keys.delete'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'moderator' AND p.name IN (
    'reviews.read',
    'reviews.update',
    'reviews.delete',
    'hotels.read',
    'hotels.update',
    'providers.read',
    'files.process',
    'files.read',
    'api_keys.create',
    'api_keys.read',
    'api_keys.delete'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'service' AND p.name IN (
    'reviews.create',
    'reviews.read',
    'reviews.update',
    'hotels.create',
    'hotels.read',
    'hotels.update',
    'providers.read',
    'files.process',
    'files.read'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Add trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_roles_updated_at BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_permissions_updated_at BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;