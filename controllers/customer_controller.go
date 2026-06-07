package controllers

import (
	"errors"

	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ─────────────────────────────────────────────
// Request/Response DTOs for Customers
// ─────────────────────────────────────────────

// CustomerResponse adalah detail customer tanpa created_at dan updated_at.
type CustomerResponse struct {
	ID              uuid.UUID `json:"id"`
	CustomerName    string    `json:"customer_name"`
	CustomerEmail   string    `json:"customer_email"`
	CustomerPhone   string    `json:"customer_phone"`
	CustomerAddress string    `json:"customer_address"`
}

// ─────────────────────────────────────────────
// Customers
// ─────────────────────────────────────────────

// GetAllCustomers godoc
// @Summary      Ambil semua customer
// @Tags         Customers
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]controllers.CustomerResponse}
// @Router       /customers [get]
// @Security     BearerAuth
func GetAllCustomers(c *fiber.Ctx) error {
	customers, err := services.GetAllCustomers()
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil data customer")
	}

	response := make([]CustomerResponse, len(customers))
	for i, cust := range customers {
		response[i] = CustomerResponse{
			ID:              cust.ID,
			CustomerName:    cust.CustomerName,
			CustomerEmail:   cust.CustomerEmail,
			CustomerPhone:   cust.CustomerPhone,
			CustomerAddress: cust.CustomerAddress,
		}
	}

	return utils.JSONSuccess(c, "Data customer berhasil diambil", response)
}

// GetCustomer godoc
// @Summary      Ambil detail customer
// @Tags         Customers
// @Param        id   path      string  true  "Customer ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=controllers.CustomerResponse}
// @Router       /customers/{id} [get]
// @Security     BearerAuth
func GetCustomer(c *fiber.Ctx) error {
	id := c.Params("id")
	customer, err := services.GetCustomerByID(id)
	if err != nil {
		if errors.Is(err, services.ErrCustomerInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrCustomerNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil detail customer")
	}

	response := CustomerResponse{
		ID:              customer.ID,
		CustomerName:    customer.CustomerName,
		CustomerEmail:   customer.CustomerEmail,
		CustomerPhone:   customer.CustomerPhone,
		CustomerAddress: customer.CustomerAddress,
	}

	return utils.JSONSuccess(c, "Detail customer berhasil diambil", response)
}

// CreateCustomer godoc
// @Summary      Tambah customer baru
// @Tags         Customers
// @Accept       json
// @Produce      json
// @Param        body  body      services.CustomerInput  true  "Data customer"
// @Success      201   {object}  utils.Response{data=models.Customer}
// @Router       /customers [post]
// @Security     BearerAuth
func CreateCustomer(c *fiber.Ctx) error {
	var input services.CustomerInput
	if err := c.BodyParser(&input); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	customer, err := services.CreateCustomer(input)
	if err != nil {
		if errors.Is(err, services.ErrCustomerDuplicateEmail) {
			return utils.JSONError(c, fiber.StatusConflict, err.Error()) // 409 Conflict
		}
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONCreated(c, "Customer berhasil ditambahkan", customer)
}

// UpdateCustomer godoc
// @Summary      Update data customer
// @Tags         Customers
// @Param        id    path      string                 true  "Customer ID"
// @Param        body  body      services.CustomerInput  true  "Data customer"
// @Produce      json
// @Success      200   {object}  utils.Response{data=models.Customer}
// @Router       /customers/{id} [put]
// @Security     BearerAuth
func UpdateCustomer(c *fiber.Ctx) error {
	id := c.Params("id")
	var input services.CustomerInput
	if err := c.BodyParser(&input); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	customer, err := services.UpdateCustomer(id, input)
	if err != nil {
		if errors.Is(err, services.ErrCustomerInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrCustomerNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		if errors.Is(err, services.ErrCustomerDuplicateEmail) {
			return utils.JSONError(c, fiber.StatusConflict, err.Error()) // 409 Conflict
		}
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Data customer berhasil diupdate", customer)
}

// DeleteCustomer godoc
// @Summary      Hapus customer
// @Tags         Customers
// @Param        id   path      string  true  "Customer ID"
// @Produce      json
// @Success      200  {object}  utils.Response
// @Router       /customers/{id} [delete]
// @Security     BearerAuth
func DeleteCustomer(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := services.DeleteCustomer(id); err != nil {
		if errors.Is(err, services.ErrCustomerInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrCustomerNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal menghapus customer")
	}
	return utils.JSONSuccess(c, "Customer berhasil dihapus", nil)
}
