package config

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectDatabase creates the database if it doesn't exist, then connects to it.
func ConnectDatabase() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	schema := os.Getenv("DB_SCHEMA")

	if schema == "" {
		schema = "public"
	}

	// Step 1: Connect to the default 'postgres' database to create target DB if needed
	defaultDSN := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable TimeZone=Asia/Jakarta",
		host, port, user, password,
	)

	adminDB, err := gorm.Open(postgres.Open(defaultDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to postgres default DB: %v", err)
	}

	// Create target database if it doesn't exist
	var count int64
	adminDB.Raw("SELECT COUNT(*) FROM pg_database WHERE datname = ?", dbName).Scan(&count)
	if count == 0 {
		adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
		log.Printf("Database '%s' created successfully.", dbName)
	}

	// Close admin connection
	sqlAdminDB, _ := adminDB.DB()
	sqlAdminDB.Close()

	// Step 2: Connect to target database
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Jakarta search_path=%s",
		host, port, user, password, dbName, schema,
	)

	logLevel := logger.Error
	if os.Getenv("NODE_ENV") == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database '%s': %v", dbName, err)
	}

	// Step 3: Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get raw DB: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(20)

	log.Printf("Connected to database '%s' successfully.", dbName)
	DB = db
}

// DropDatabase terminates all active connections to the target database and drops it.
func DropDatabase() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbName == "" {
		return fmt.Errorf("DB_NAME environment variable is empty")
	}

	// Close the current DB pool if it exists
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil && sqlDB != nil {
			sqlDB.Close()
			log.Println("Closed existing database connection pool.")
		}
	}

	// Connect to default 'postgres' database
	defaultDSN := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable TimeZone=Asia/Jakarta",
		host, port, user, password,
	)

	adminDB, err := gorm.Open(postgres.Open(defaultDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to postgres default DB: %v", err)
	}
	defer func() {
		sqlAdminDB, _ := adminDB.DB()
		if sqlAdminDB != nil {
			sqlAdminDB.Close()
		}
	}()

	// Terminate active connections to the target database
	terminateQuery := fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s' AND pid <> pg_backend_pid()`, dbName)

	if err := adminDB.Exec(terminateQuery).Error; err != nil {
		log.Printf("Warning: failed to terminate active connections: %v", err)
	}

	// Drop the target database
	dropQuery := fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)
	if err := adminDB.Exec(dropQuery).Error; err != nil {
		return fmt.Errorf("failed to drop database '%s': %v", dbName, err)
	}

	log.Printf("Database '%s' dropped successfully.", dbName)
	return nil
}

