package services

import (
	"errors"
	"fmt"
	"strings"

	"teknik/config"
	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for Customer service (prefixed to avoid package-level namespace clashes)
var (
	ErrCustomerInvalidUUID      = errors.New("ID customer tidak valid")
	ErrCustomerNotFound         = errors.New("customer tidak ditemukan")
	ErrCustomerDuplicateEmail   = errors.New("email customer sudah terdaftar")
)

type CustomerInput struct {
	CustomerName    string `json:"customer_name"`
	CustomerEmail   string `json:"customer_email"`
	CustomerPhone   string `json:"customer_phone"`
	CustomerAddress string `json:"customer_address"`
}

// GetAllCustomers mengambil daftar seluruh customer.
func GetAllCustomers() ([]models.Customer, error) {
	var customers []models.Customer
	if err := config.DB.Select("id, customer_name, customer_email, customer_phone, customer_address").Find(&customers).Error; err != nil {
		return nil, err
	}
	if customers == nil {
		customers = []models.Customer{}
	}
	return customers, nil
}

// GetCustomerByID mengambil detail customer berdasarkan ID.
func GetCustomerByID(id string) (models.Customer, error) {
	if _, err := uuid.Parse(id); err != nil {
		return models.Customer{}, ErrCustomerInvalidUUID
	}

	var customer models.Customer
	err := config.DB.First(&customer, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Customer{}, ErrCustomerNotFound
		}
		return models.Customer{}, err
	}
	return customer, nil
}

// CreateCustomer membuat data customer baru.
func CreateCustomer(input CustomerInput) (models.Customer, error) {
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	if input.CustomerName == "" {
		return models.Customer{}, fmt.Errorf("nama customer tidak boleh kosong")
	}

	input.CustomerEmail = strings.TrimSpace(strings.ToLower(input.CustomerEmail))
	if !utils.IsValidEmail(input.CustomerEmail) {
		return models.Customer{}, fmt.Errorf("format email tidak valid")
	}

	// Cek apakah email sudah terdaftar
	var count int64
	if err := config.DB.Model(&models.Customer{}).Where("customer_email = ?", input.CustomerEmail).Count(&count).Error; err != nil {
		return models.Customer{}, err
	}
	if count > 0 {
		return models.Customer{}, ErrCustomerDuplicateEmail
	}

	customer := models.Customer{
		ID:              uuid.New(),
		CustomerName:    input.CustomerName,
		CustomerEmail:   input.CustomerEmail,
		CustomerPhone:   strings.TrimSpace(input.CustomerPhone),
		CustomerAddress: strings.TrimSpace(input.CustomerAddress),
	}

	if err := config.DB.Create(&customer).Error; err != nil {
		return models.Customer{}, err
	}

	return customer, nil
}

// UpdateCustomer memperbarui data customer yang sudah ada.
func UpdateCustomer(id string, input CustomerInput) (models.Customer, error) {
	customer, err := GetCustomerByID(id)
	if err != nil {
		return models.Customer{}, err
	}

	input.CustomerName = strings.TrimSpace(input.CustomerName)
	if input.CustomerName == "" {
		return models.Customer{}, fmt.Errorf("nama customer tidak boleh kosong")
	}

	if input.CustomerEmail != "" {
		input.CustomerEmail = strings.TrimSpace(strings.ToLower(input.CustomerEmail))
		if !utils.IsValidEmail(input.CustomerEmail) {
			return models.Customer{}, fmt.Errorf("format email tidak valid")
		}

		// Cek duplikasi email diluar ID customer yang sedang diupdate
		var count int64
		if err := config.DB.Model(&models.Customer{}).
			Where("customer_email = ? AND id != ?", input.CustomerEmail, customer.ID).
			Count(&count).Error; err != nil {
			return models.Customer{}, err
		}
		if count > 0 {
			return models.Customer{}, ErrCustomerDuplicateEmail
		}
		customer.CustomerEmail = input.CustomerEmail
	}

	customer.CustomerName = input.CustomerName
	customer.CustomerPhone = strings.TrimSpace(input.CustomerPhone)
	customer.CustomerAddress = strings.TrimSpace(input.CustomerAddress)

	if err := config.DB.Save(&customer).Error; err != nil {
		return models.Customer{}, err
	}

	return customer, nil
}

// DeleteCustomer menghapus data customer berdasarkan ID.
func DeleteCustomer(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrCustomerInvalidUUID
	}

	tx := config.DB.Delete(&models.Customer{}, "id = ?", id)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return ErrCustomerNotFound
	}
	return nil
}
