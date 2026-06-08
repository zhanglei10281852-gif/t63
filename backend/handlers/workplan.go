package handlers

import (
	"fmt"
	"net/http"
	"sanitation-backend/database"
	"sanitation-backend/models"
	"sanitation-backend/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

func GenerateDailyPlans(date string) error {
	var roads []models.RoadSection
	database.DB.Find(&roads)

	var existingCount int64
	database.DB.Model(&models.WorkPlan{}).Where("plan_date = ?", date).Count(&existingCount)
	if existingCount > 0 {
		return nil
	}

	for _, road := range roads {
		var times []string
		switch road.Level {
		case 1:
			times = []string{"06:00", "13:00", "18:00"}
		case 2:
			times = []string{"06:00", "18:00"}
		case 3:
			times = []string{"06:00"}
		default:
			times = []string{"06:00"}
		}

		var workers []models.Worker
		database.DB.Where("road_section_id = ? AND status = ?", road.ID, "active").Find(&workers)

		for i, planTime := range times {
			var workerID *uint
			if len(workers) > 0 {
				workerIndex := i % len(workers)
				workerID = &workers[workerIndex].ID
			}

			plan := models.WorkPlan{
				RoadSectionID: road.ID,
				WorkerID:      workerID,
				PlanDate:      date,
				PlanTime:      planTime,
				Status:        "pending",
			}
			database.DB.Create(&plan)
		}
	}

	return nil
}

func GetTodayPlans(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	GenerateDailyPlans(date)

	var plans []models.WorkPlan
	query := database.DB.Preload("RoadSection").Preload("Worker").Where("plan_date = ?", date)

	if user.Role == "area_manager" && user.AreaID != nil {
		var roadIDs []uint
		var roads []models.RoadSection
		database.DB.Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			query = query.Where("road_section_id IN ?", roadIDs)
		}
		_ = roads
	}

	status := c.Query("status")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Order("plan_time ASC").Find(&plans)
	c.JSON(http.StatusOK, plans)
}

func UpdatePlanWorker(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	id := c.Param("id")

	var plan models.WorkPlan
	if err := database.DB.First(&plan, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if user.Role == "area_manager" && user.AreaID != nil {
		var road models.RoadSection
		database.DB.First(&road, plan.RoadSectionID)
		if road.AreaID != *user.AreaID {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他片区任务"})
			return
		}
	}

	var req struct {
		WorkerID *uint `json:"worker_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if req.WorkerID != nil {
		var worker models.Worker
		if err := database.DB.First(&worker, *req.WorkerID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "工人不存在"})
			return
		}
	}

	database.DB.Model(&plan).Update("worker_id", req.WorkerID)
	database.DB.Preload("Worker").Preload("RoadSection").First(&plan, id)

	c.JSON(http.StatusOK, plan)
}

type CheckinRequest struct {
	PlanID     uint   `json:"plan_id" binding:"required"`
	WorkMethod string `json:"work_method"`
	CheckinTime string `json:"checkin_time"`
}

func Checkin(c *gin.Context) {
	var req CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	var plan models.WorkPlan
	if err := database.DB.First(&plan, req.PlanID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if plan.Status == "completed" || plan.Status == "late" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该任务已打卡"})
		return
	}

	var checkinTime time.Time
	if req.CheckinTime != "" {
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", req.CheckinTime, time.Local)
		if err == nil {
			checkinTime = parsed
		} else {
			checkinTime = time.Now()
		}
	} else {
		checkinTime = time.Now()
	}

	planDateTime := plan.PlanDate + " " + plan.PlanTime + ":00"
	planTime, err := time.ParseInLocation("2006-01-02 15:04:05", planDateTime, time.Local)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "计划时间解析失败"})
		return
	}

	diff := checkinTime.Sub(planTime).Minutes()
	status := "completed"
	if diff < -30 || diff > 30 {
		status = "late"
	}

	workMethod := req.WorkMethod
	if workMethod == "" {
		workMethod = "manual"
	}

	database.DB.Model(&plan).Updates(map[string]interface{}{
		"status":       status,
		"work_method":  workMethod,
		"checkin_time": checkinTime,
	})

	database.DB.Preload("Worker").Preload("RoadSection").First(&plan, plan.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "打卡成功",
		"status":  status,
		"plan":    plan,
	})
}

func GetCheckinRecords(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var plans []models.WorkPlan
	query := database.DB.Preload("RoadSection").Preload("Worker").
		Where("plan_date = ? AND (status = ? OR status = ?)", date, "completed", "late").
		Order("checkin_time DESC")

	if user.Role == "area_manager" && user.AreaID != nil {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			query = query.Where("road_section_id IN ?", roadIDs)
		}
	}

	query.Find(&plans)
	c.JSON(http.StatusOK, plans)
}

func MarkMissedTasks() {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	database.DB.Model(&models.WorkPlan{}).
		Where("plan_date = ? AND status = ?", yesterday, "pending").
		Update("status", "missed")
	fmt.Println("Marked missed tasks for", yesterday)
}
