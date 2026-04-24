package utils

import "github.com/gofiber/fiber/v2"

// Response adalah standard wrapper untuk semua API response.
type Response struct {
	Success bool        `json:"success" example:"true"`
	Message string      `json:"message" example:"Operasi berhasil"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONSuccess sends a 200 OK JSON response.
func JSONSuccess(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// JSONCreated sends a 201 Created JSON response.
func JSONCreated(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// JSONError sends an error JSON response with the given status code.
func JSONError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(Response{
		Success: false,
		Message: message,
	})
}
