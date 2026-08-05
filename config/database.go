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

// ConnectDatabase membuat database jika belum ada, kemudian menghubungkannya.
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

	// Langkah 1: Hubungkan ke default database 'postgres' untuk membuat database target jika diperlukan
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

	// Buat database target jika belum ada
	var count int64
	if err := adminDB.Raw("SELECT COUNT(*) FROM pg_database WHERE datname = ?", dbName).Scan(&count).Error; err != nil {
		log.Printf("Warning: failed to check if database exists: %v", err)
	}
	if count == 0 {
		if err := adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)).Error; err != nil {
			log.Fatalf("Failed to create database '%s': %v", dbName, err)
		}
		log.Printf("Database '%s' created successfully.", dbName)
	}

	// Tutup koneksi admin
	if sqlAdminDB, err := adminDB.DB(); err == nil && sqlAdminDB != nil {
		sqlAdminDB.Close()
	}

	// Langkah 2: Hubungkan ke database target
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

	// Langkah 3: Konfigurasi pool koneksi
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get raw DB: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(20)

	log.Printf("Connected to database '%s' successfully.", dbName)
	DB = db
}

// DropDatabase memutuskan semua koneksi aktif ke database target dan menghapusnya.
func DropDatabase() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbName == "" {
		return fmt.Errorf("DB_NAME environment variable is empty")
	}

	// Tutup pool DB saat ini jika ada
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil && sqlDB != nil {
			sqlDB.Close()
			log.Println("Closed existing database connection pool.")
		}
	}

	// Hubungkan ke database default 'postgres'
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
		if sqlAdminDB, err := adminDB.DB(); err == nil && sqlAdminDB != nil {
			sqlAdminDB.Close()
		}
	}()

	// Putuskan koneksi aktif ke database target
	terminateQuery := fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s' AND pid <> pg_backend_pid()`, dbName)

	if err := adminDB.Exec(terminateQuery).Error; err != nil {
		log.Printf("Warning: failed to terminate active connections: %v", err)
	}

	// Hapus database target
	dropQuery := fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)
	if err := adminDB.Exec(dropQuery).Error; err != nil {
		return fmt.Errorf("failed to drop database '%s': %v", dbName, err)
	}

	log.Printf("Database '%s' dropped successfully.", dbName)
	return nil
}

