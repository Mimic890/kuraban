package handlers

import (
	"Mimic890/kuraban/internal/services"

	"github.com/gofiber/fiber/v3"
)

func getAdminService(c fiber.Ctx) services.AdminService {
	return c.Locals("adminService").(services.AdminService)
}

func getS3Service(c fiber.Ctx) services.S3Storage {
	return c.Locals("s3Service").(services.S3Storage)
}
