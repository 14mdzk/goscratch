// Package casbin provides the Casbin RBAC adapter and its NoOp variant.
//
// # Production use
//
// Use [NewAdapter] with a real PostgreSQL DSN. When authorization is enabled in
// the application config, [app.New] will hard-fail if the adapter cannot be
// initialised — the NoOp is NOT used as a fallback (see ADR-006 carve-out).
//
// # NoOpAdapter
//
// [NoOpAdapter] is intended for use in unit tests and for when authorization is
// explicitly disabled in the application config. It permits every [Enforce]
// call unconditionally. Do NOT wire it into production code paths where
// authorization is expected to be enforced — doing so silently opens every
// authenticated endpoint.
package casbin
