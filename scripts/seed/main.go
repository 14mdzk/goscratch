package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"golang.org/x/crypto/bcrypt"
)

// Default seed users
var seedUsers = []struct {
	Email    string
	Password string
	Name     string
	Role     string
}{
	{
		Email:    "superadmin@example.com",
		Password: "superadmin123",
		Name:     "Super Admin",
		Role:     "superadmin",
	},
	{
		Email:    "admin@example.com",
		Password: "admin123",
		Name:     "Admin User",
		Role:     "admin",
	},
	{
		Email:    "user@example.com",
		Password: "user123",
		Name:     "Normal User",
		Role:     "viewer",
	},
	// Pagination test users (25+ users for testing cursor pagination)
	{Email: "alice.johnson@example.com", Password: "test123", Name: "Alice Johnson", Role: "viewer"},
	{Email: "bob.smith@example.com", Password: "test123", Name: "Bob Smith", Role: "viewer"},
	{Email: "carol.williams@example.com", Password: "test123", Name: "Carol Williams", Role: "viewer"},
	{Email: "david.brown@example.com", Password: "test123", Name: "David Brown", Role: "viewer"},
	{Email: "emma.davis@example.com", Password: "test123", Name: "Emma Davis", Role: "viewer"},
	{Email: "frank.miller@example.com", Password: "test123", Name: "Frank Miller", Role: "viewer"},
	{Email: "grace.wilson@example.com", Password: "test123", Name: "Grace Wilson", Role: "viewer"},
	{Email: "henry.moore@example.com", Password: "test123", Name: "Henry Moore", Role: "viewer"},
	{Email: "irene.taylor@example.com", Password: "test123", Name: "Irene Taylor", Role: "viewer"},
	{Email: "jack.anderson@example.com", Password: "test123", Name: "Jack Anderson", Role: "viewer"},
	{Email: "kate.thomas@example.com", Password: "test123", Name: "Kate Thomas", Role: "viewer"},
	{Email: "liam.jackson@example.com", Password: "test123", Name: "Liam Jackson", Role: "viewer"},
	{Email: "mia.white@example.com", Password: "test123", Name: "Mia White", Role: "viewer"},
	{Email: "noah.harris@example.com", Password: "test123", Name: "Noah Harris", Role: "viewer"},
	{Email: "olivia.martin@example.com", Password: "test123", Name: "Olivia Martin", Role: "viewer"},
	{Email: "peter.clark@example.com", Password: "test123", Name: "Peter Clark", Role: "viewer"},
	{Email: "quinn.lewis@example.com", Password: "test123", Name: "Quinn Lewis", Role: "viewer"},
	{Email: "rachel.walker@example.com", Password: "test123", Name: "Rachel Walker", Role: "viewer"},
	{Email: "sam.hall@example.com", Password: "test123", Name: "Sam Hall", Role: "viewer"},
	{Email: "tina.allen@example.com", Password: "test123", Name: "Tina Allen", Role: "viewer"},
	{Email: "ulysses.young@example.com", Password: "test123", Name: "Ulysses Young", Role: "viewer"},
	{Email: "victoria.king@example.com", Password: "test123", Name: "Victoria King", Role: "viewer"},
	{Email: "william.wright@example.com", Password: "test123", Name: "William Wright", Role: "viewer"},
	{Email: "xena.lopez@example.com", Password: "test123", Name: "Xena Lopez", Role: "viewer"},
	{Email: "yusuf.hill@example.com", Password: "test123", Name: "Yusuf Hill", Role: "viewer"},
	{Email: "zara.scott@example.com", Password: "test123", Name: "Zara Scott", Role: "viewer"},
}

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ctx := context.Background()

	// Connect to database
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	fmt.Println("🌱 Starting database seeding...")

	for _, u := range seedUsers {
		// Check if user already exists
		var exists bool
		var userID string
		err := pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", u.Email).Scan(&userID)
		if err == nil {
			exists = true
		}

		if exists {
			fmt.Printf("⏭️  User %s already exists, checking role assignment...\\n", u.Email)
		} else {
			// Hash password
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("⚠️  Error hashing password for %s: %v", u.Email, err)
				continue
			}

			// Insert user and get ID
			err = pool.QueryRow(ctx,
				"INSERT INTO users (email, password_hash, name, is_active) VALUES ($1, $2, $3, true) RETURNING id",
				u.Email, string(passwordHash), u.Name,
			).Scan(&userID)
			if err != nil {
				log.Printf("⚠️  Error creating user %s: %v", u.Email, err)
				continue
			}

			fmt.Printf("✅ Created user: %s\\n", u.Email)
		}

		// Assign role via Casbin (g, user_id, role)
		if userID != "" && u.Role != "" {
			// Check if role assignment already exists
			var roleExists bool
			err := pool.QueryRow(ctx,
				"SELECT EXISTS(SELECT 1 FROM casbin_rules WHERE p_type = 'g' AND v0 = $1 AND v1 = $2)",
				userID, u.Role,
			).Scan(&roleExists)
			if err != nil {
				log.Printf("⚠️  Error checking role for %s: %v", u.Email, err)
				continue
			}

			if roleExists {
				fmt.Printf("⏭️  Role '%s' already assigned to %s\\n", u.Role, u.Email)
			} else {
				// Insert role assignment
				_, err = pool.Exec(ctx,
					"INSERT INTO casbin_rules (p_type, v0, v1) VALUES ('g', $1, $2)",
					userID, u.Role,
				)
				if err != nil {
					log.Printf("⚠️  Error assigning role to %s: %v", u.Email, err)
					continue
				}
				fmt.Printf("✅ Assigned role '%s' to %s\\n", u.Role, u.Email)
			}
		}
	}

	fmt.Println("\n🎉 Database seeding completed!")
	fmt.Println("\n📋 Seeded Users:")
	fmt.Println("┌────────────────────────────┬───────────────┬────────────┐")
	fmt.Println("│ Email                      │ Password      │ Role       │")
	fmt.Println("├────────────────────────────┼───────────────┼────────────┤")
	for _, u := range seedUsers {
		fmt.Printf("│ %-26s │ %-13s │ %-10s │\n", u.Email, u.Password, u.Role)
	}
	fmt.Println("└────────────────────────────┴───────────────┴────────────┘")
}
