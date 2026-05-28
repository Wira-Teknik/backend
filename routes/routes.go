package routes

import (
	"teknik/controllers"
	"teknik/middlewares"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes registers all API routes.
func SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	// Auth routes (public)
	auth := api.Group("/auth")
	auth.Post("/register", controllers.Register)
	auth.Post("/login", controllers.Login)
	auth.Post("/forgot-password/request", controllers.ForgotPasswordRequest)
	auth.Post("/forgot-password/verify", controllers.ForgotPasswordVerify)
	auth.Post("/forgot-password/reset", controllers.ForgotPasswordReset)

	// ─── Protected routes ────────────────────────
	protected := api.Group("", middlewares.RequireAuth)

	// Dashboard routes
	protected.Get("/dashboard", controllers.GetDashboard)
	protected.Get("/dashboard/activities", controllers.GetAllDashboardActivities)

	// Customer routes
	customers := protected.Group("/customers")
	customers.Get("/", controllers.GetAllCustomers)
	customers.Get("/:id", controllers.GetCustomer)
	customers.Post("/", controllers.CreateCustomer)
	customers.Put("/:id", controllers.UpdateCustomer)
	customers.Delete("/:id", controllers.DeleteCustomer)

	// Order routes
	orders := protected.Group("/orders")
	orders.Get("/", controllers.GetAllOrders)
	orders.Get("/next-trx", controllers.GetNextTransactionNo)
	orders.Get("/:id", controllers.GetOrder)
	orders.Post("/", controllers.CreateOrder)
	orders.Put("/:id", controllers.UpdateOrder)
	orders.Delete("/:id", controllers.DeleteOrder)
	orders.Get("/:orderId/shipments", controllers.GetShipmentsByOrder)

	// Shipment routes
	shipments := protected.Group("/shipments")
	shipments.Get("/:id", controllers.GetShipment)
	shipments.Post("/", controllers.CreateShipment)
	shipments.Patch("/:id/received", controllers.ConfirmShipmentReceived)
	shipments.Get("/:shipmentId/invoice", controllers.GetInvoiceByShipment)

	// Invoice routes
	invoices := protected.Group("/invoices")
	invoices.Get("/", controllers.GetAllInvoices)
	invoices.Get("/:id", controllers.GetInvoice)

	// Payment routes
	payments := protected.Group("/payments")
	payments.Get("/", controllers.GetAllPayments)
	payments.Get("/history", controllers.GetPaymentHistory)
	payments.Get("/history/export", controllers.ExportPaymentHistory)
	payments.Get("/customer/:name", controllers.GetCustomerPaymentDetail)
	payments.Get("/:id", controllers.GetPayment)
	payments.Post("/", controllers.CreatePayment)
	payments.Put("/:id", controllers.UpdatePaymentTotal)

	// Attachment routes
	attachments := protected.Group("/attachments")
	attachments.Get("/:relatedId", controllers.GetAttachments)
	attachments.Post("/", controllers.UploadAttachment)

	// Payment Recap routes
	recap := protected.Group("/payment-recap")
	recap.Get("/", controllers.GetPaymentRecapSummary)
	recap.Get("/detail-pendapatan", controllers.GetDetailPendapatan)
	recap.Get("/detail-pendapatan/export", controllers.ExportDetailPendapatan)
	recap.Get("/detail-pesanan", controllers.GetDetailPesanan)
	recap.Get("/detail-pesanan/export", controllers.ExportDetailPesanan)
	recap.Get("/detail-unpaid", controllers.GetDetailUnpaid)
	recap.Get("/detail-unpaid/export", controllers.ExportDetailUnpaid)
	recap.Get("/detail-paid", controllers.GetDetailPaid)
	recap.Get("/detail-paid/export", controllers.ExportDetailPaid)

	// Audit routes (Owner only)
	audit := protected.Group("/audit", middlewares.RequireRole("owner"))
	audit.Get("/", controllers.GetAuditLogs)
}
