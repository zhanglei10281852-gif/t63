package handlers

import (
	"fmt"
	"net/http"
	"sanitation-backend/database"
	"sanitation-backend/models"
	"time"

	"github.com/gin-gonic/gin"
)

func GetVehicles(c *gin.Context) {
	var vehicles []models.Vehicle
	status := c.Query("status")
	query := database.DB

	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Find(&vehicles)
	c.JSON(http.StatusOK, vehicles)
}

func GetVehicleRecords(c *gin.Context) {
	vehicleID := c.Query("vehicle_id")
	var records []models.VehicleRecord
	query := database.DB.Preload("Vehicle").Preload("Driver")

	if vehicleID != "" {
		query = query.Where("vehicle_id = ?", vehicleID)
	}

	query.Order("created_at DESC").Limit(100).Find(&records)
	c.JSON(http.StatusOK, records)
}

func StartVehicle(c *gin.Context) {
	var req struct {
		VehicleID uint  `json:"vehicle_id" binding:"required"`
		DriverID  *uint `json:"driver_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var vehicle models.Vehicle
	if err := database.DB.First(&vehicle, req.VehicleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "车辆不存在"})
		return
	}

	if vehicle.Status != "available" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "车辆当前状态不可出车"})
		return
	}

	now := time.Now()
	record := models.VehicleRecord{
		VehicleID:  req.VehicleID,
		DriverID:   req.DriverID,
		DepartTime: &now,
	}

	if err := database.DB.Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建出车记录失败"})
		return
	}

	database.DB.Model(&vehicle).Update("status", "on_duty")

	c.JSON(http.StatusOK, gin.H{
		"message": "出车成功",
		"record":  record,
	})
}

func ReturnVehicle(c *gin.Context) {
	var req struct {
		RecordID        *uint   `json:"record_id"`
		VehicleID       *uint   `json:"vehicle_id"`
		Mileage         float64 `json:"mileage"`
		FuelConsumption float64 `json:"fuel_consumption"`
		RoadSectionIDs  string  `json:"road_section_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var record models.VehicleRecord
	var err error

	if req.RecordID != nil {
		err = database.DB.First(&record, *req.RecordID).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "出车记录不存在"})
			return
		}
	} else if req.VehicleID != nil {
		err = database.DB.Where("vehicle_id = ? AND return_time IS NULL", *req.VehicleID).
			Order("depart_time DESC").
			First(&record).Error
		if err != nil {
			var vehicle models.Vehicle
			if vErr := database.DB.First(&vehicle, *req.VehicleID).Error; vErr != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "车辆不存在"})
				return
			}
			if vehicle.Status != "on_duty" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "车辆未处于出车状态"})
				return
			}
			now := time.Now().Add(-time.Hour)
			record = models.VehicleRecord{
				VehicleID:  *req.VehicleID,
				DepartTime: &now,
			}
			database.DB.Create(&record)
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供record_id或vehicle_id"})
		return
	}

	if record.ReturnTime != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该车辆已收车"})
		return
	}

	now := time.Now()
	database.DB.Model(&record).Updates(map[string]interface{}{
		"return_time":       now,
		"mileage":           req.Mileage,
		"fuel_consumption":  req.FuelConsumption,
		"road_section_ids":  req.RoadSectionIDs,
	})

	var vehicle models.Vehicle
	database.DB.First(&vehicle, record.VehicleID)
	newMileage := vehicle.Mileage + req.Mileage
	database.DB.Model(&vehicle).Updates(map[string]interface{}{
		"status":  "available",
		"mileage": newMileage,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "收车成功",
		"record":  record,
	})
}

func GetMaintenanceReminders(c *gin.Context) {
	var vehicles []models.Vehicle
	database.DB.Find(&vehicles)

	var reminders []gin.H
	now := time.Now()

	for _, v := range vehicles {
		needMaintenance := false
		reason := ""

		mileageSinceMaintenance := v.Mileage - v.LastMaintenanceMileage
		if mileageSinceMaintenance >= 5000 {
			needMaintenance = true
			reason = fmt.Sprintf("距上次保养已行驶%.0f公里，超过5000公里保养阈值", mileageSinceMaintenance)
		}

		if v.LastMaintenance != nil {
			daysSinceMaintenance := now.Sub(*v.LastMaintenance).Hours() / 24
			if daysSinceMaintenance >= 30 {
				needMaintenance = true
				if reason != "" {
					reason += "；"
				}
				reason += fmt.Sprintf("距上次保养已%.0f天，超过30天保养周期", daysSinceMaintenance)
			}
		}

		if needMaintenance {
			reminders = append(reminders, gin.H{
				"vehicle_id":   v.ID,
				"plate_number": v.PlateNumber,
				"type":         v.Type,
				"status":       v.Status,
				"mileage":      v.Mileage,
				"reason":       reason,
			})
		}
	}

	c.JSON(http.StatusOK, reminders)
}

func UpdateVehicleStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var vehicle models.Vehicle
	if err := database.DB.First(&vehicle, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "车辆不存在"})
		return
	}

	if req.Status == "available" && vehicle.Status == "maintenance" {
		now := time.Now()
		database.DB.Model(&vehicle).Updates(map[string]interface{}{
			"status":                  req.Status,
			"last_maintenance":        now,
			"last_maintenance_mileage": vehicle.Mileage,
		})
	} else {
		database.DB.Model(&vehicle).Update("status", req.Status)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "状态更新成功",
		"vehicle": vehicle,
	})
}
