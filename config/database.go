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
