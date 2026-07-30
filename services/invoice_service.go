package services

import (
	"errors"

	"teknik/config"
	"teknik/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for Invoice service
var (
	ErrInvoiceInvalidUUID       = errors.New("ID invoice tidak valid")
	ErrInvoiceInvalidShipmentID = errors.New("ID pengiriman tidak valid")
	ErrInvoiceNotFound          = errors.New("invoice tidak ditemukan")
)

// ─────────────────────────────────────────────
// Get All Invoices
// ─────────────────────────────────────────────

// GetAllInvoices mengambil seluruh data invoice dari database.
func GetAllInvoices() ([]models.Invoice, error) {
	var invoices []models.Invoice
	err := config.DB.Order("created_at DESC").Find(&invoices).Error
	if invoices == nil {
		invoices = []models.Invoice{}
	}
	return invoices, err
}

// ─────────────────────────────────────────────
// Get Invoice By ID
// ─────────────────────────────────────────────

// GetInvoiceByID mengambil detail invoice berdasarkan ID.
func GetInvoiceByID(id string) (models.Invoice, error) {
	if _, err := uuid.Parse(id); err != nil {
		return models.Invoice{}, ErrInvoiceInvalidUUID
	}

	var invoice models.Invoice
	err := config.DB.First(&invoice, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Invoice{}, ErrInvoiceNotFound
		}
		return models.Invoice{}, err
	}
	return invoice, nil
}

// ─────────────────────────────────────────────
// Get Invoices by Shipment ID
// ─────────────────────────────────────────────

// GetInvoiceByShipmentID mengambil invoice yang terkait dengan pengiriman barang tertentu.
func GetInvoiceByShipmentID(shipmentID string) (models.Invoice, error) {
	if _, err := uuid.Parse(shipmentID); err != nil {
		return models.Invoice{}, ErrInvoiceInvalidShipmentID
	}

	var invoice models.Invoice
	err := config.DB.Where("shipment_id = ?", shipmentID).First(&invoice).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Invoice{}, ErrInvoiceNotFound
		}
		return models.Invoice{}, err
	}
	return invoice, nil
}
