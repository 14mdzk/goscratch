-- Admin permission to query the audit-log read endpoint (Tier E4).
-- Superadmin is already covered by its ('superadmin', '*', '*') wildcard.
INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', 'audit', 'read')
ON CONFLICT (p_type, v0, v1, v2, v3, v4, v5) DO NOTHING;
