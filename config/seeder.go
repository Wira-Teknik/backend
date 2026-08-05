package config

import (
	"fmt"
	"log"
	"os"
)

// ResetDatabase menghapus skema database saat ini dan membuatnya kembali.
func ResetDatabase() {
	if DB == nil {
		log.Println("Database connection is not initialized.")
		return
	}

	schema := os.Getenv("DB_SCHEMA")
	if schema == "" {
		schema = "public"
	}
	log.Printf("Resetting database by dropping schema '%s'...", schema)

	if err := DB.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)).Error; err != nil {
		log.Printf("Failed to drop schema '%s': %v", schema, err)
		return
	}
	if err := DB.Exec(fmt.Sprintf("CREATE SCHEMA %s", schema)).Error; err != nil {
		log.Printf("Failed to recreate schema '%s': %v", schema, err)
		return
	}
	log.Println("Database schema reset successfully.")
}

