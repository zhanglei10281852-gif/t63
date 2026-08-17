package main

import (
	"log"
	"os"

	"sanitation-backend/database"
	"sanitation-backend/handlers"
	vehiclerepository "sanitation-backend/internal/vehicle/repository"
	vehicleservice "sanitation-backend/internal/vehicle/service"
	"sanitation-backend/middleware"
	"sanitation-backend/models"
	"sanitation-backend/seed"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	database.InitDB()

	if err := models.AutoMigrate(database.DB); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migrated successfully")

	seed.SeedData()
	vehicleRepository := vehiclerepository.NewGORM(database.DB)
	vehicleService := vehicleservice.New(vehicleRepository, vehicleservice.SystemClock{})
	vehicleHandler := handlers.NewVehicleHandler(vehicleService)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		api.POST("/auth/login", handlers.Login)

		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.GET("/auth/me", handlers.GetCurrentUserInfo)

			auth.GET("/areas", handlers.GetAreas)
			auth.GET("/road-sections", handlers.GetRoadSections)
			auth.GET("/road-sections/:id", handlers.GetRoadSectionByID)
			auth.GET("/workers", handlers.GetWorkers)

			auth.GET("/dashboard", handlers.GetDashboardStats)

			work := auth.Group("/work-plans")
			{
				work.GET("", handlers.GetTodayPlans)
				work.PUT("/:id/worker", middleware.AreaManagerOrAdmin(), handlers.UpdatePlanWorker)
			}

			auth.POST("/checkin", handlers.Checkin)
			auth.GET("/checkin-records", handlers.GetCheckinRecords)

			inspection := auth.Group("/inspections")
			{
				inspection.GET("", handlers.GetInspections)
				inspection.POST("", middleware.AreaManagerOrAdmin(), handlers.CreateInspection)
			}

			vehicle := auth.Group("/vehicles")
			{
				vehicle.GET("", handlers.GetVehicles)
				vehicle.GET("/records", handlers.GetVehicleRecords)
				vehicle.GET("/maintenance-reminders", handlers.GetMaintenanceReminders)
				vehicle.POST("/start", vehicleHandler.StartVehicle)
				vehicle.POST("/return", vehicleHandler.ReturnVehicle)
				vehicle.PUT("/:id/status", middleware.AdminRequired(), vehicleHandler.UpdateVehicleStatus)
			}

			complaint := auth.Group("/complaints")
			{
				complaint.GET("", handlers.GetComplaints)
				complaint.POST("", handlers.CreateComplaint)
				complaint.PUT("/:id/handle", middleware.AreaManagerOrAdmin(), handlers.HandleComplaint)
				complaint.PUT("/:id/assign", middleware.AdminRequired(), handlers.AssignComplaint)
			}

			auth.GET("/assessment/monthly", handlers.GetMonthlyAssessment)
		}
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8653"
	}

	log.Printf("Server starting on port %s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
