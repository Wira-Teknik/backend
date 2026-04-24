package services

import (
	"fmt"
	"strings"

	"teknik/config"
	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
)

type CustomerInput struct {
	CustomerName    string `json:"customer_name"`
	CustomerEmail   string `json:"customer_email"`
	CustomerPhone   string `json:"customer_phone"`
	CustomerAddress string `json:"customer_address"`
}

func GetAllCustomers() ([]models.Customer, error) {
	var customers []models.Customer
	// Hanya mengambil kolom yang diperlukan
	err := config.DB.Select("id, customer_name, customer_email, customer_phone, customer_address").Find(&customers).Error
	return customers, err
}

func GetCustomerByID(id string) (models.Customer, error) {
	var customer models.Customer
	err := config.DB.First(&customer, "id = ?", id).Error
	return customer, err
}

func CreateCustomer(input CustomerInput) (models.Customer, error) {
	input.CustomerEmail = strings.TrimSpace(strings.ToLower(input.CustomerEmail))

	if !utils.IsValidEmail(input.CustomerEmail) {
		return models.Customer{}, fmt.Errorf("format email tidak valid")
	}

	customer := models.Customer{
		ID:              uuid.New(),
		CustomerName:    input.CustomerName,
		CustomerEmail:   input.CustomerEmail,
		CustomerPhone:   input.CustomerPhone,
		CustomerAddress: input.CustomerAddress,
	}

	if err := config.DB.Create(&customer).Error; err != nil {
		return models.Customer{}, err
	}

	return customer, nil
}

func UpdateCustomer(id string, input CustomerInput) (models.Customer, error) {
	customer, err := GetCustomerByID(id)
	if err != nil {
		return models.Customer{}, fmt.Errorf("customer tidak ditemukan")
	}

	if input.CustomerEmail != "" {
		input.CustomerEmail = strings.TrimSpace(strings.ToLower(input.CustomerEmail))
		if !utils.IsValidEmail(input.CustomerEmail) {
			return models.Customer{}, fmt.Errorf("format email tidak valid")
		}
		customer.CustomerEmail = input.CustomerEmail
	}

	customer.CustomerName = input.CustomerName
	customer.CustomerPhone = input.CustomerPhone
	customer.CustomerAddress = input.CustomerAddress

	if err := config.DB.Save(&customer).Error; err != nil {
		return models.Customer{}, err
	}

	return customer, nil
}

func DeleteCustomer(id string) error {
	return config.DB.Delete(&models.Customer{}, "id = ?", id).Error
}
