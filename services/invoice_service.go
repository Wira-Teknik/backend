package services

import (
	"fmt"

	"teknik/config"
	"teknik/models"
)

// ─────────────────────────────────────────────
// Get All Invoices
// ─────────────────────────────────────────────

func GetAllInvoices() ([]models.Invoice, error) {
	var invoices []models.Invoice
	err := config.DB.Order("created_at DESC").Find(&invoices).Error
	return invoices, err
}

// ─────────────────────────────────────────────
// Get Invoice By ID
// ─────────────────────────────────────────────

func GetInvoiceByID(id string) (models.Invoice, error) {
	var invoice models.Invoice
	err := config.DB.First(&invoice, "id = ?", id).Error
	return invoice, err
}

// ─────────────────────────────────────────────
// Get Invoices by Shipment ID
// ─────────────────────────────────────────────

func GetInvoiceByShipmentID(shipmentID string) (models.Invoice, error) {
	var invoice models.Invoice
	err := config.DB.Where("shipment_id = ?", shipmentID).First(&invoice).Error
	if err != nil {
		return models.Invoice{}, fmt.Errorf("invoice tidak ditemukan untuk pengiriman ini")
	}
	return invoice, nil
}
