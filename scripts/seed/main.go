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
