CREATE TABLE IF NOT EXISTS casbin_rules (
    id SERIAL PRIMARY KEY,
    p_type VARCHAR(100) NOT NULL DEFAULT '',
    v0 VARCHAR(100) NOT NULL DEFAULT '',
    v1 VARCHAR(100) NOT NULL DEFAULT '',
    v2 VARCHAR(100) NOT NULL DEFAULT '',
    v3 VARCHAR(100) NOT NULL DEFAULT '',
    v4 VARCHAR(100) NOT NULL DEFAULT '',
    v5 VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_casbin_rules_p_type ON casbin_rules(p_type);
CREATE INDEX IF NOT EXISTS idx_casbin_rules_v0 ON casbin_rules(v0);
CREATE INDEX IF NOT EXISTS idx_casbin_rules_v1 ON casbin_rules(v1);

-- Seed default role permissions
-- Format: p, role, object, action

-- Superadmin has all permissions
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'superadmin', '*', '*');

-- Admin permissions
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'read');
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'create');
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'update');
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'delete');

-- Editor permissions
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'editor', 'users', 'read');
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'editor', 'users', 'update');

-- Viewer permissions
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'viewer', 'users', 'read');
