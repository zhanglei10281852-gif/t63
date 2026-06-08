package handlers

import (
	"net/http"
	"sanitation-backend/database"
	"sanitation-backend/models"
	"sanitation-backend/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

func GetComplaints(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var complaints []models.Complaint

	query := database.DB.Preload("RoadSection").Preload("AssignedUser")

	if user.Role == "area_manager" && user.AreaID != nil {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			query = query.Where("road_section_id IN ?", roadIDs)
		}
	}

	status := c.Query("status")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Order("created_at DESC").Find(&complaints)
	c.JSON(http.StatusOK, complaints)
}

type CreateComplaintRequest struct {
	Content          string `json:"content" binding:"required"`
	RoadSectionID    uint   `json:"road_section_id" binding:"required"`
	Complainant      string `json:"complainant"`
	ComplainantPhone string `json:"complainant_phone"`
}

func CreateComplaint(c *gin.Context) {
	var req CreateComplaintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	var road models.RoadSection
	if err := database.DB.First(&road, req.RoadSectionID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路段不存在"})
		return
	}

	var areaManager models.User
	database.DB.Where("role = ? AND area_id = ?", "area_manager", road.AreaID).First(&areaManager)

	var assignedTo *uint
	if areaManager.ID > 0 {
		assignedTo = &areaManager.ID
	}

	complaint := models.Complaint{
		Content:          req.Content,
		RoadSectionID:    req.RoadSectionID,
		ComplaintTime:    time.Now(),
		Complainant:      req.Complainant,
		ComplainantPhone: req.ComplainantPhone,
		Status:           "pending",
		AssignedTo:       assignedTo,
	}

	if err := database.DB.Create(&complaint).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建投诉失败"})
		return
	}

	database.DB.Preload("RoadSection").Preload("AssignedUser").First(&complaint, complaint.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "投诉提交成功",
		"data":    complaint,
	})
}

type HandleComplaintRequest struct {
	HandleResult string `json:"handle_result" binding:"required"`
	IsValid      bool   `json:"is_valid"`
}

func HandleComplaint(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	id := c.Param("id")

	var complaint models.Complaint
	if err := database.DB.First(&complaint, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "投诉不存在"})
		return
	}

	if user.Role == "area_manager" && user.AreaID != nil {
		var road models.RoadSection
		database.DB.First(&road, complaint.RoadSectionID)
		if road.AreaID != *user.AreaID {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权处理其他片区投诉"})
			return
		}
	}

	var req HandleComplaintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	now := time.Now()
	status := "resolved"
	if !req.IsValid {
		status = "invalid"
	}

	database.DB.Model(&complaint).Updates(map[string]interface{}{
		"status":        status,
		"handle_result": req.HandleResult,
		"handle_time":   now,
		"is_valid":      req.IsValid,
	})

	database.DB.Preload("RoadSection").Preload("AssignedUser").First(&complaint, id)

	c.JSON(http.StatusOK, gin.H{
		"message": "投诉处理完成",
		"data":    complaint,
	})
}

func AssignComplaint(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		AssignedTo uint `json:"assigned_to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var complaint models.Complaint
	if err := database.DB.First(&complaint, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "投诉不存在"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, req.AssignedTo).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "指派用户不存在"})
		return
	}

	database.DB.Model(&complaint).Updates(map[string]interface{}{
		"assigned_to": req.AssignedTo,
		"status":      "processing",
	})

	database.DB.Preload("RoadSection").Preload("AssignedUser").First(&complaint, id)

	c.JSON(http.StatusOK, gin.H{
		"message": "投诉已指派",
		"data":    complaint,
	})
}
