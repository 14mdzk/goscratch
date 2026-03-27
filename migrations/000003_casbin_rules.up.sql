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
CREATE UNIQUE INDEX IF NOT EXISTS idx_casbin_rules_unique ON casbin_rules(p_type, v0, v1, v2, v3, v4, v5);

-- Seed default role permissions
-- Format: p, role, object, action

-- Superadmin has all permissions (wildcard)
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'superadmin', '*', '*')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;

-- Admin permissions
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'create')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'update')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'users', 'delete')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'roles', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'roles', 'assign')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'roles', 'manage')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'files', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'files', 'upload')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'files', 'delete')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'jobs', 'dispatch')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;

-- Editor permissions
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'editor', 'users', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'editor', 'users', 'update')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'editor', 'files', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'editor', 'files', 'upload')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;

-- Viewer permissions
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'viewer', 'users', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'viewer', 'files', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
