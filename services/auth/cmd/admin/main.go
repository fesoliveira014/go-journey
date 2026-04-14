package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"

	pkgdb "github.com/fesoliveira014/library-system/pkg/db"
	"github.com/fesoliveira014/library-system/services/auth/internal/model"
)

func main() {
	email := flag.String("email", "", "admin email (required)")
	password := flag.String("password", "", "admin password (required)")
	name := flag.String("name", "", "admin display name (required)")
	flag.Parse()

	if *email == "" || *password == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "Usage: admin --email EMAIL --password PASSWORD --name NAME")
		fmt.Fprintln(os.Stderr, "Requires DATABASE_URL environment variable")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := pkgdb.Open(dsn, pkgdb.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}
	hashStr := string(hash)

	var existing model.User
	result := db.Where("email = ?", *email).First(&existing)
	if result.Error == nil {
		existing.Role = "admin"
		existing.PasswordHash = &hashStr
		existing.Name = *name
		if err := db.Save(&existing).Error; err != nil {
			log.Fatalf("failed to update user: %v", err)
		}
		fmt.Printf("Updated existing user %s to admin role\n", *email)
		return
	}

	user := model.User{
		Email:        *email,
		PasswordHash: &hashStr,
		Name:         *name,
		Role:         "admin",
	}
	if err := db.Create(&user).Error; err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}
	fmt.Printf("Created admin user: %s (%s)\n", *email, user.ID)
}
