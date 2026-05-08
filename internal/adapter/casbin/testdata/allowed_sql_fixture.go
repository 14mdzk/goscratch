//go:build ignore

// Package testdata contains fixtures used by scripts/lint-casbin-sql.sh to
// verify that raw-SQL writes inside internal/adapter/casbin/ are NOT flagged
// by the guard. Do not import this package.
package testdata

// allowedInsert is an example of legitimate SQL that lives inside the casbin
// adapter package and must NOT be flagged by the lint guard.
const allowedInsert = `INSERT INTO casbin_rules (p_type, v0, v1, v2) VALUES ('p', 'admin', '*', '*')`
