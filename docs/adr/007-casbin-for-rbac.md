# ADR-007: Casbin for RBAC

## Status
Accepted

## Context
We needed role-based access control with the ability to define roles, assign permissions to roles, and check permissions at the middleware level. Options were: a custom RBAC implementation with database tables, or an established policy engine like Casbin or Open Policy Agent (OPA).

## Decision
We chose Casbin v3 with a PostgreSQL policy adapter. Casbin provides a mature, well-tested RBAC model with built-in support for role hierarchies, policy persistence, and efficient enforcement. It is wrapped behind the `port.Authorizer` interface so it can be swapped without affecting business logic.

The permission model uses three-tuple policies: `(subject, object, action)` -- e.g., `(admin, users, read)`. Four predefined roles are provided: superadmin, admin, editor, viewer.

## Consequences
- **Pro:** Battle-tested policy engine with extensive documentation
- **Pro:** Policies stored in PostgreSQL -- no separate policy server needed
- **Pro:** Supports role inheritance and implicit permissions out of the box
- **Pro:** Hidden behind `port.Authorizer` interface -- can be replaced with OPA or custom logic
- **Con:** Casbin's API and model configuration have a learning curve
- **Con:** Adds a dependency; a simple custom RBAC table might suffice for small projects
