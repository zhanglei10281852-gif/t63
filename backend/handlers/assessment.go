package handlers

import (
	"net/http"
	"sanitation-backend/database"
	"sanitation-backend/models"
	"sanitation-backend/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

func GetDashboardStats(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	today := time.Now().Format("2006-01-02")
	currentMonth := time.Now().Format("2006-01")

	GenerateDailyPlans(today)

	var totalTasks int64
	var completedTasks int64

	totalQuery := database.DB.Model(&models.WorkPlan{}).Where("plan_date = ?", today)
	completedQuery := database.DB.Model(&models.WorkPlan{}).Where("plan_date = ? AND (status = ? OR status = ?)", today, "completed", "late")

	if user.Role == "area_manager" && user.AreaID != nil {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			totalQuery = totalQuery.Where("road_section_id IN ?", roadIDs)
			completedQuery = completedQuery.Where("road_section_id IN ?", roadIDs)
		}
	}

	totalQuery.Count(&totalTasks)
	completedQuery.Count(&completedTasks)

	completionRate := 0.0
	if totalTasks > 0 {
		completionRate = float64(completedTasks) / float64(totalTasks) * 100
	}

	var areas []models.Area
	database.DB.Find(&areas)

	areaStats := make([]gin.H, 0)
	for _, area := range areas {
		if user.Role == "area_manager" && user.AreaID != nil && area.ID != *user.AreaID {
			continue
		}

		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", area.ID).Pluck("id", &roadIDs)

		var areaTotal int64
		var areaCompleted int64

		if len(roadIDs) > 0 {
			database.DB.Model(&models.WorkPlan{}).Where("plan_date = ? AND road_section_id IN ?", today, roadIDs).Count(&areaTotal)
			database.DB.Model(&models.WorkPlan{}).Where("plan_date = ? AND road_section_id IN ? AND (status = ? OR status = ?)", today, roadIDs, "completed", "late").Count(&areaCompleted)
		}

		rate := 0.0
		if areaTotal > 0 {
			rate = float64(areaCompleted) / float64(areaTotal) * 100
		}

		areaStats = append(areaStats, gin.H{
			"area_id":         area.ID,
			"area_name":       area.Name,
			"total_tasks":     areaTotal,
			"completed_tasks": areaCompleted,
			"completion_rate": rate,
		})
	}

	monthlyQualityTrend := make([]gin.H, 0)
	for i := 29; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		var avgScore float64

		query := database.DB.Model(&models.QualityInspection{}).Where("DATE(inspect_time) = ?", date)
		if user.Role == "area_manager" && user.AreaID != nil {
			var roadIDs []uint
			database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
			if len(roadIDs) > 0 {
				query = query.Where("road_section_id IN ?", roadIDs)
			}
		}

		query.Select("COALESCE(AVG(score), 0)").Scan(&avgScore)
		monthlyQualityTrend = append(monthlyQualityTrend, gin.H{
			"date":  date,
			"score": avgScore,
		})
	}

	var pendingComplaints int64
	complaintQuery := database.DB.Model(&models.Complaint{}).Where("status = ?", "pending")
	if user.Role == "area_manager" && user.AreaID != nil {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			complaintQuery = complaintQuery.Where("road_section_id IN ?", roadIDs)
		}
	}
	complaintQuery.Count(&pendingComplaints)

	var monthlyAvgScore float64
	monthlyScoreQuery := database.DB.Model(&models.QualityInspection{}).
		Where("to_char(inspect_time, 'YYYY-MM') = ?", currentMonth)
	if user.Role == "area_manager" && user.AreaID != nil {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", *user.AreaID).Pluck("id", &roadIDs)
		if len(roadIDs) > 0 {
			monthlyScoreQuery = monthlyScoreQuery.Where("road_section_id IN ?", roadIDs)
		}
	}
	monthlyScoreQuery.Select("COALESCE(AVG(score), 100)").Scan(&monthlyAvgScore)

	c.JSON(http.StatusOK, gin.H{
		"today_completion_rate":   completionRate,
		"today_total_tasks":       totalTasks,
		"today_completed_tasks":   completedTasks,
		"area_stats":              areaStats,
		"monthly_quality_trend":   monthlyQualityTrend,
		"pending_complaints":      pendingComplaints,
		"monthly_avg_score":       monthlyAvgScore,
		"current_month":           currentMonth,
	})
}

func CalculateMonthlyAssessment(month string) {
	var areas []models.Area
	database.DB.Find(&areas)

	for _, area := range areas {
		var roadIDs []uint
		database.DB.Model(&models.RoadSection{}).Where("area_id = ?", area.ID).Pluck("id", &roadIDs)

		var qualityScore float64
		if len(roadIDs) > 0 {
			database.DB.Model(&models.QualityInspection{}).
				Where("road_section_id IN ? AND to_char(inspect_time, 'YYYY-MM') = ?", roadIDs, month).
				Select("COALESCE(AVG(score), 100)").Scan(&qualityScore)
		} else {
			qualityScore = 100
		}

		var totalTasks int64
		var completedTasks int64
		if len(roadIDs) > 0 {
			database.DB.Model(&models.WorkPlan{}).
				Where("road_section_id IN ? AND to_char(plan_date::date, 'YYYY-MM') = ?", roadIDs, month).
				Count(&totalTasks)
			database.DB.Model(&models.WorkPlan{}).
				Where("road_section_id IN ? AND to_char(plan_date::date, 'YYYY-MM') = ? AND (status = ? OR status = ?)", roadIDs, month, "completed", "late").
				Count(&completedTasks)
		}

		completionRate := 0.0
		if totalTasks > 0 {
			completionRate = float64(completedTasks) / float64(totalTasks) * 100
		}

		var validComplaints int64
		if len(roadIDs) > 0 {
			database.DB.Model(&models.Complaint{}).
				Where("road_section_id IN ? AND is_valid = ? AND to_char(complaint_time, 'YYYY-MM') = ?", roadIDs, true, month).
				Count(&validComplaints)
		}
		complaintDeduction := int(validComplaints) * 5

		totalScore := qualityScore*0.5 + completionRate*0.3 - float64(complaintDeduction)
		if totalScore < 0 {
			totalScore = 0
		}

		grade := calculateGrade(totalScore)

		var assessment models.MonthlyAssessment
		database.DB.Where("month = ? AND target_type = ? AND target_id = ?", month, "area", area.ID).First(&assessment)

		if assessment.ID == 0 {
			assessment = models.MonthlyAssessment{
				Month:              month,
				TargetType:         "area",
				TargetID:           area.ID,
				TargetName:         area.Name,
				AreaID:             &area.ID,
				CompletionRate:     completionRate,
				QualityScore:       qualityScore,
				ComplaintDeduction: complaintDeduction,
				TotalScore:         totalScore,
				Grade:              grade,
			}
			database.DB.Create(&assessment)
		} else {
			database.DB.Model(&assessment).Updates(map[string]interface{}{
				"completion_rate":     completionRate,
				"quality_score":       qualityScore,
				"complaint_deduction": complaintDeduction,
				"total_score":         totalScore,
				"grade":               grade,
			})
		}
	}

	var workers []models.Worker
	database.DB.Find(&workers)

	for _, worker := range workers {
		var totalTasks int64
		var completedTasks int64

		database.DB.Model(&models.WorkPlan{}).
			Where("worker_id = ? AND to_char(plan_date::date, 'YYYY-MM') = ?", worker.ID, month).
			Count(&totalTasks)
		database.DB.Model(&models.WorkPlan{}).
			Where("worker_id = ? AND to_char(plan_date::date, 'YYYY-MM') = ? AND (status = ? OR status = ?)", worker.ID, month, "completed", "late").
			Count(&completedTasks)

		completionRate := 0.0
		if totalTasks > 0 {
			completionRate = float64(completedTasks) / float64(totalTasks) * 100
		}

		var qualityScore float64
		var workerRoadID *uint
		if worker.RoadSectionID != nil {
			workerRoadID = worker.RoadSectionID
		}

		if workerRoadID != nil {
			database.DB.Model(&models.QualityInspection{}).
				Where("road_section_id = ? AND to_char(inspect_time, 'YYYY-MM') = ?", *workerRoadID, month).
				Select("COALESCE(AVG(score), 100)").Scan(&qualityScore)
		} else {
			qualityScore = 100
		}

		completionPart := completionRate * 0.6
		qualityPart := qualityScore * 0.4
		totalScore := completionPart + qualityPart
		if totalScore > 100 {
			totalScore = 100
		}

		grade := calculateGrade(totalScore)

		var assessment models.MonthlyAssessment
		database.DB.Where("month = ? AND target_type = ? AND target_id = ?", month, "worker", worker.ID).First(&assessment)

		areaID := worker.AreaID

		if assessment.ID == 0 {
			assessment = models.MonthlyAssessment{
				Month:          month,
				TargetType:     "worker",
				TargetID:       worker.ID,
				TargetName:     worker.Name,
				AreaID:         &areaID,
				CompletionRate: completionRate,
				QualityScore:   qualityScore,
				TotalScore:     totalScore,
				Grade:          grade,
			}
			database.DB.Create(&assessment)
		} else {
			database.DB.Model(&assessment).Updates(map[string]interface{}{
				"completion_rate": completionRate,
				"quality_score":   qualityScore,
				"total_score":     totalScore,
				"grade":           grade,
			})
		}
	}
}

func calculateGrade(score float64) string {
	if score >= 90 {
		return "excellent"
	} else if score >= 75 {
		return "good"
	} else if score >= 60 {
		return "pass"
	}
	return "fail"
}

func GetMonthlyAssessment(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	month := c.Query("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	CalculateMonthlyAssessment(month)

	targetType := c.Query("target_type")
	if targetType == "" {
		targetType = "area"
	}

	var assessments []models.MonthlyAssessment
	query := database.DB.Where("month = ? AND target_type = ?", month, targetType)

	if user.Role == "area_manager" && user.AreaID != nil {
		query = query.Where("area_id = ?", *user.AreaID)
	}

	query.Order("total_score DESC").Find(&assessments)

	c.JSON(http.StatusOK, assessments)
}
