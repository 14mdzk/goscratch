package casbin

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	sqladapter "github.com/Blank-Xu/sql-adapter"
	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx stdlib driver

	"github.com/14mdzk/goscratch/internal/port"
)

// Adapter implements port.Authorizer using Casbin
type Adapter struct {
	enforcer *casbin.Enforcer
	db       *sql.DB
}

// Config holds configuration for the Casbin adapter
type Config struct {
	DatabaseURL string
	ModelText   string // Optional: inline model text (if not using file)
}

// Default RBAC model with permission-based enforcement
// Uses simple equality matching with wildcard (*) support
const defaultModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`

// NewAdapter creates a new Casbin adapter
func NewAdapter(cfg Config) (*Adapter, error) {
	// Open database connection using pgx stdlib
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create SQL adapter for Casbin
	adapter, err := sqladapter.NewAdapter(db, "postgres", "casbin_rules")
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin adapter: %w", err)
	}

	// Parse model
	modelText := cfg.ModelText
	if modelText == "" {
		modelText = defaultModel
	}

	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin model: %w", err)
	}

	// Create enforcer
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	// Load policies from database
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load policies: %w", err)
	}

	// Log loaded policies
	policies, _ := enforcer.GetPolicy()
	slog.Info("Casbin policies loaded", "count", len(policies))
	for _, p := range policies {
		slog.Debug("Policy", "rule", p)
	}

	groupings, _ := enforcer.GetGroupingPolicy()
	slog.Info("Casbin role assignments loaded", "count", len(groupings))
	for _, g := range groupings {
		slog.Debug("Role assignment", "rule", g)
	}

	return &Adapter{
		enforcer: enforcer,
		db:       db,
	}, nil
}

// Enforce checks if subject has permission to perform action on object
func (a *Adapter) Enforce(sub, obj, act string) (bool, error) {
	allowed, err := a.enforcer.Enforce(sub, obj, act)
	slog.Debug("Casbin enforce", "sub", sub, "obj", obj, "act", act, "allowed", allowed, "error", err)
	return allowed, err
}

// EnforceWithContext checks permission with context
func (a *Adapter) EnforceWithContext(ctx context.Context, sub, obj, act string) (bool, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return a.enforcer.Enforce(sub, obj, act)
	}
}

// AddRoleForUser assigns a role to a user
func (a *Adapter) AddRoleForUser(userID, role string) error {
	_, err := a.enforcer.AddGroupingPolicy(userID, role)
	return err
}

// RemoveRoleForUser removes a role from a user
func (a *Adapter) RemoveRoleForUser(userID, role string) error {
	_, err := a.enforcer.RemoveGroupingPolicy(userID, role)
	return err
}

// GetRolesForUser returns all roles for a user
func (a *Adapter) GetRolesForUser(userID string) ([]string, error) {
	return a.enforcer.GetRolesForUser(userID)
}

// GetUsersForRole returns all users with a given role
func (a *Adapter) GetUsersForRole(role string) ([]string, error) {
	return a.enforcer.GetUsersForRole(role)
}

// HasRoleForUser checks if a user has a specific role
func (a *Adapter) HasRoleForUser(userID, role string) (bool, error) {
	return a.enforcer.HasRoleForUser(userID, role)
}

// AddPermissionForRole adds a permission to a role
func (a *Adapter) AddPermissionForRole(role, obj, act string) error {
	_, err := a.enforcer.AddPolicy(role, obj, act)
	return err
}

// RemovePermissionForRole removes a permission from a role
func (a *Adapter) RemovePermissionForRole(role, obj, act string) error {
	_, err := a.enforcer.RemovePolicy(role, obj, act)
	return err
}

// GetPermissionsForRole returns all permissions for a role
func (a *Adapter) GetPermissionsForRole(role string) ([][]string, error) {
	return a.enforcer.GetPermissionsForUser(role)
}

// AddPermissionForUser adds a direct permission to a user
func (a *Adapter) AddPermissionForUser(userID, obj, act string) error {
	_, err := a.enforcer.AddPolicy(userID, obj, act)
	return err
}

// RemovePermissionForUser removes a direct permission from a user
func (a *Adapter) RemovePermissionForUser(userID, obj, act string) error {
	_, err := a.enforcer.RemovePolicy(userID, obj, act)
	return err
}

// GetPermissionsForUser returns direct permissions for a user
func (a *Adapter) GetPermissionsForUser(userID string) ([][]string, error) {
	return a.enforcer.GetPermissionsForUser(userID)
}

// GetImplicitPermissionsForUser returns all permissions including via roles
func (a *Adapter) GetImplicitPermissionsForUser(userID string) ([][]string, error) {
	return a.enforcer.GetImplicitPermissionsForUser(userID)
}

// LoadPolicy reloads policies from database
func (a *Adapter) LoadPolicy() error {
	return a.enforcer.LoadPolicy()
}

// SavePolicy saves policies to database
func (a *Adapter) SavePolicy() error {
	return a.enforcer.SavePolicy()
}

// Close closes the database connection
func (a *Adapter) Close() error {
	return a.db.Close()
}

// BuildDatabaseURL constructs a PostgreSQL connection string
func BuildDatabaseURL(host string, port int, user, password, dbname string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		user, password, host, port, dbname)
}

// Ensure Adapter implements port.Authorizer
var _ port.Authorizer = (*Adapter)(nil)

// Helper function to format permission string
func FormatPermission(obj, act string) string {
	return fmt.Sprintf("%s:%s", obj, act)
}

// ParsePermission splits a permission string into object and action
func ParsePermission(perm string) (obj, act string) {
	parts := strings.SplitN(perm, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return perm, "*"
}
