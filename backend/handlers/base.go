package handlers

import (
	"net/http"
	"sanitation-backend/database"
	"sanitation-backend/models"
	"sanitation-backend/middleware"

	"github.com/gin-gonic/gin"
)

func GetAreas(c *gin.Context) {
	var areas []models.Area
	database.DB.Find(&areas)
	c.JSON(http.StatusOK, areas)
}

func GetRoadSections(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var roads []models.RoadSection

	query := database.DB.Preload("Area")
	if user.Role == "area_manager" && user.AreaID != nil {
		query = query.Where("area_id = ?", *user.AreaID)
	}

	query.Find(&roads)
	c.JSON(http.StatusOK, roads)
}

func GetWorkers(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var workers []models.Worker

	query := database.DB.Preload("Area").Preload("RoadSection")
	if user.Role == "area_manager" && user.AreaID != nil {
		query = query.Where("area_id = ?", *user.AreaID)
	}

	query.Find(&workers)
	c.JSON(http.StatusOK, workers)
}

func GetRoadSectionByID(c *gin.Context) {
	id := c.Param("id")
	var road models.RoadSection
	if err := database.DB.Preload("Area").First(&road, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "路段不存在"})
		return
	}
	c.JSON(http.StatusOK, road)
}
