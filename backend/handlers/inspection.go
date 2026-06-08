package handlers

import (
	"encoding/json"
	"net/http"
	"sanitation-backend/database"
	"sanitation-backend/models"
	"sanitation-backend/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

type CreateInspectionRequest struct {
	RoadSectionID uint                `json:"road_section_id" binding:"required"`
	InspectTime   string              `json:"inspect_time"`
	Deductions    []models.DeductionItem `json:"deductions"`
	Remark        string              `json:"remark"`
}

func CreateInspection(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req CreateInspectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	var road models.RoadSection
	if err := database.DB.First(&road, req.RoadSectionID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路段不存在"})
		return
	}

	if user.Role == "area_manager" && user.AreaID != nil {
		if road.AreaID != *user.AreaID {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权抽查其他片区路段"})
			return
		}
	}

	inspectTime := time.Now()
	if req.InspectTime != "" {
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", req.InspectTime, time.Local); err == nil {
			inspectTime = parsed
		}
	}

	totalDeduction := 0
	for _, d := range req.Deductions {
		totalDeduction += d.Score
	}
	score := 100 - totalDeduction
	if score < 0 {
		score = 0
	}

	deductionsJSON, _ := json.Marshal(req.Deductions)

	inspection := models.QualityInspection{
		RoadSectionID: req.RoadSectionID,
		InspectorID:   user.ID,
		InspectTime:   inspectTime,
		Score:         score,
		Deductions:    string(deductionsJSON),
		Remark:        req.Remark,
	}

	if err := database.DB.Create(&inspection).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建抽查记录失败"})
		return
	}

	database.DB.Preload("RoadSection").Preload("Inspector").First(&inspection, inspection.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "抽查记录创建成功",
		"data":    inspection,
	})
}

func GetInspections(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var inspections []models.QualityInspection
	query := database.DB.Preload("RoadSection").Preload("Inspector")

	if user.Role == "area_manager" && user.AreaID != nil {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			query = query.Where("road_section_id IN ?", roadIDs)
		}
	}

	roadID := c.Query("road_section_id")
	if roadID != "" {
		query = query.Where("road_section_id = ?", roadID)
	}

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	if startDate != "" && endDate != "" {
		query = query.Where("inspect_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	month := c.Query("month")
	if month != "" {
		query = query.Where("to_char(inspect_time, 'YYYY-MM') = ?", month)
	}

	query.Order("inspect_time DESC").Find(&inspections)

	for i := range inspections {
		if inspections[i].Deductions != "" {
			var deductions []models.DeductionItem
			json.Unmarshal([]byte(inspections[i].Deductions), &deductions)
		}
	}

	c.JSON(http.StatusOK, inspections)
}

func GetMonthlyAvgScore(month string, areaID *uint) float64 {
	var avgScore float64
	query := database.DB.Model(&models.QualityInspection{}).
		Where("to_char(inspect_time, 'YYYY-MM') = ?", month)

	if areaID != nil {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *areaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			query = query.Where("road_section_id IN ?", roadIDs)
		}
	}

	query.Select("COALESCE(AVG(score), 100)").Scan(&avgScore)
	return avgScore
}
