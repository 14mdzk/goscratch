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
	Role     string // For future Casbin integration
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
		Role:     "user",
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
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", u.Email).Scan(&exists)
		if err != nil {
			log.Printf("⚠️  Error checking user %s: %v", u.Email, err)
			continue
		}

		if exists {
			fmt.Printf("⏭️  User %s already exists, skipping\n", u.Email)
			continue
		}

		// Hash password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("⚠️  Error hashing password for %s: %v", u.Email, err)
			continue
		}

		// Insert user
		_, err = pool.Exec(ctx,
			"INSERT INTO users (email, password_hash, name, is_active) VALUES ($1, $2, $3, true)",
			u.Email, string(passwordHash), u.Name,
		)
		if err != nil {
			log.Printf("⚠️  Error creating user %s: %v", u.Email, err)
			continue
		}

		fmt.Printf("✅ Created user: %s (role: %s)\n", u.Email, u.Role)
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
