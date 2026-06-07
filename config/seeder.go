package config

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
)

type seedOrderItem struct {
	ProductName string  `json:"product_name"`
	OrderQty    int     `json:"order_qty"`
	UnitPrice   float64 `json:"unit_price"`
}

type seedOrder struct {
	TransactionNo    string          `json:"transaction_no"`
	PoNo             string          `json:"po_no"`
	OrderDate        string          `json:"order_date"`
	RecipientName    string          `json:"recipient_name"`
	RecipientAddress string          `json:"recipient_address"`
	RecipientPhone   string          `json:"recipient_phone"`
	RecipientEmail   string          `json:"recipient_email"`
	Items            []seedOrderItem `json:"items"`
}

// SeedOrders seeds the orders table from order-seed.json if the table is empty.
func SeedOrders() {
	var count int64
	if err := DB.Unscoped().Model(&models.Order{}).Count(&count).Error; err != nil {
		log.Printf("Failed to count orders for seeding: %v", err)
		return
	}

	if count > 0 {
		return // Already seeded
	}

	log.Println("Seeding database with default orders from config/order-seed.json...")

	filePath := "config/order-seed.json"
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read seed file %s: %v", filePath, err)
		return
	}

	var seedOrders []seedOrder
	if err := json.Unmarshal(data, &seedOrders); err != nil {
		log.Printf("Failed to unmarshal seed data: %v", err)
		return
	}

	const ppnRate = 0.11

	tx := DB.Begin()
	for _, so := range seedOrders {
		orderDate, err := time.Parse("2006-01-02", so.OrderDate)
		if err != nil {
			log.Printf("Warning: failed to parse order date '%s' for order %s: %v", so.OrderDate, so.TransactionNo, err)
			continue
		}

		// Check if transaction number already exists (unscoped) to prevent unique key violation
		var existingCount int64
		if err := tx.Unscoped().Model(&models.Order{}).Where("transaction_no = ?", so.TransactionNo).Count(&existingCount).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to check existing order %s: %v", so.TransactionNo, err)
			return
		}
		if existingCount > 0 {
			log.Printf("Order %s already exists (possibly soft-deleted), skipping...", so.TransactionNo)
			continue
		}

		orderID := uuid.New()
		var items []models.OrderItem

		for _, item := range so.Items {
			ppn := mathRoundTwo(float64(item.OrderQty) * item.UnitPrice * ppnRate)
			subtotal := mathRoundTwo(float64(item.OrderQty)*item.UnitPrice + ppn)

			items = append(items, models.OrderItem{
				ID:           uuid.New(),
				OrderID:      orderID,
				ProductName:  item.ProductName,
				OrderQty:     item.OrderQty,
				RemainingQty: item.OrderQty,
				UnitPrice:    item.UnitPrice,
				PPN:          ppn,
				Subtotal:     subtotal,
			})
		}

		order := models.Order{
			ID:               orderID,
			TransactionNo:    so.TransactionNo,
			PoNo:             so.PoNo,
			OrderDate:        utils.JSONDate(orderDate),
			RecipientName:    so.RecipientName,
			RecipientAddress: so.RecipientAddress,
			RecipientPhone:   so.RecipientPhone,
			RecipientEmail:   so.RecipientEmail,
			OrderStatus:      models.OrderStatusPending,
			Items:            items,
		}

		if err := tx.Create(&order).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to seed order %s: %v", so.TransactionNo, err)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit seed transaction: %v", err)
		return
	}

	log.Printf("Successfully seeded %d orders into the database.", len(seedOrders))
}

func mathRoundTwo(val float64) float64 {
	return math.Round(val*100) / 100
}

// ResetDatabase drops the database schema and recreates it.
func ResetDatabase() {
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
