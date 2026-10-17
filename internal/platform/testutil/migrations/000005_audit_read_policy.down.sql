DELETE FROM casbin_rules
WHERE p_type = 'p' AND v0 = 'admin' AND v1 = 'audit' AND v2 = 'read';
