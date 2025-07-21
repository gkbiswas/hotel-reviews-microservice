package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate [up|down|status]")
		os.Exit(1)
	}

	command := os.Args[1]

	// Get database configuration from environment
	dbHost := getEnv("HOTEL_REVIEWS_DATABASE_HOST", "localhost")
	dbPort := getEnv("HOTEL_REVIEWS_DATABASE_PORT", "5432")
	dbUser := getEnv("HOTEL_REVIEWS_DATABASE_USER", "postgres")
	dbPassword := getEnv("HOTEL_REVIEWS_DATABASE_PASSWORD", "postgres")
	dbName := getEnv("HOTEL_REVIEWS_DATABASE_NAME", "hotel_reviews_test")

	// First, try to create the database if it doesn't exist
	if err := createDatabaseIfNotExists(dbHost, dbPort, dbUser, dbPassword, dbName); err != nil {
		log.Printf("Warning: Could not create database: %v", err)
	}

	// Build connection string
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Connect to database
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		log.Fatal("Failed to create migrations table:", err)
	}

	// Execute command
	switch command {
	case "up":
		if err := migrateUp(db); err != nil {
			log.Fatal("Migration failed:", err)
		}
		fmt.Println("Migrations completed successfully")
	case "down":
		if err := migrateDown(db); err != nil {
			log.Fatal("Rollback failed:", err)
		}
		fmt.Println("Rollback completed successfully")
	case "status":
		if err := showStatus(db); err != nil {
			log.Fatal("Failed to show status:", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func createDatabaseIfNotExists(host, port, user, password, dbName string) error {
	// Connect to postgres database to create the target database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		host, port, user, password)
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// Check if database exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			return err
		}
		fmt.Printf("Created database: %s\n", dbName)
	} else {
		fmt.Printf("Database already exists: %s\n", dbName)
	}

	return nil
}

func createMigrationsTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		id SERIAL PRIMARY KEY,
		migration_name VARCHAR(255) NOT NULL UNIQUE,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	
	_, err := db.Exec(query)
	return err
}

func migrateUp(db *sql.DB) error {
	// Get list of migration files
	migrationsDir := "migrations"
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return err
	}

	// Filter out rollback files and sort
	var migrationFiles []string
	for _, file := range files {
		if !strings.Contains(file, "_rollback.sql") {
			migrationFiles = append(migrationFiles, file)
		}
	}
	sort.Strings(migrationFiles)

	// Get applied migrations
	appliedMigrations := make(map[string]bool)
	rows, err := db.Query("SELECT migration_name FROM schema_migrations")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		appliedMigrations[name] = true
	}

	// Apply pending migrations
	for _, file := range migrationFiles {
		migrationName := filepath.Base(file)
		migrationName = strings.TrimSuffix(migrationName, ".sql")

		if appliedMigrations[migrationName] {
			fmt.Printf("Skipping already applied migration: %s\n", migrationName)
			continue
		}

		fmt.Printf("Applying migration: %s\n", migrationName)

		// Read migration file
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		// Begin transaction
		tx, err := db.Begin()
		if err != nil {
			return err
		}

		// Apply migration
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", migrationName, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (migration_name) VALUES ($1)", migrationName); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migrationName, err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return err
		}

		fmt.Printf("Applied migration: %s\n", migrationName)
	}

	return nil
}

func migrateDown(db *sql.DB) error {
	// This is a simplified version - in production you'd want rollback files
	return fmt.Errorf("rollback not implemented")
}

func showStatus(db *sql.DB) error {
	fmt.Println("Applied migrations:")
	
	rows, err := db.Query("SELECT migration_name, applied_at FROM schema_migrations ORDER BY applied_at")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var appliedAt sql.NullTime
		if err := rows.Scan(&name, &appliedAt); err != nil {
			return err
		}
		fmt.Printf("  %s - %s\n", name, appliedAt.Time.Format("2006-01-02 15:04:05"))
	}

	return rows.Err()
}