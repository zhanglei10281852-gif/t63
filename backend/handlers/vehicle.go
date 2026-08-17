package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sanitation-backend/database"
	"sanitation-backend/internal/vehicle/domain"
	"sanitation-backend/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type VehicleOperations interface {
	Start(context.Context, domain.StartRequest) (domain.Record, error)
	Return(context.Context, domain.ReturnRequest) (domain.Record, error)
	UpdateStatus(context.Context, domain.UpdateStatusRequest) (domain.Vehicle, error)
}

type VehicleHandler struct {
	service VehicleOperations
}

func NewVehicleHandler(service VehicleOperations) *VehicleHandler {
	return &VehicleHandler{service: service}
}

func GetVehicles(c *gin.Context) {
	var vehicles []models.Vehicle
	status := c.Query("status")
	query := database.DB.WithContext(c.Request.Context())

	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Find(&vehicles)
	c.JSON(http.StatusOK, vehicles)
}

func GetVehicleRecords(c *gin.Context) {
	vehicleID := c.Query("vehicle_id")
	var records []models.VehicleRecord
	query := database.DB.WithContext(c.Request.Context()).Preload("Vehicle").Preload("Driver")

	if vehicleID != "" {
		query = query.Where("vehicle_id = ?", vehicleID)
	}

	query.Order("created_at DESC").Limit(100).Find(&records)
	c.JSON(http.StatusOK, records)
}

func (h *VehicleHandler) StartVehicle(c *gin.Context) {
	var req struct {
		VehicleID uint  `json:"vehicle_id" binding:"required"`
		DriverID  *uint `json:"driver_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	record, err := h.service.Start(c.Request.Context(), domain.StartRequest{
		VehicleID: req.VehicleID,
		DriverID:  req.DriverID,
	})
	if err != nil {
		writeVehicleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "出车成功",
		"record":  vehicleRecordResponse(record),
	})
}

func (h *VehicleHandler) ReturnVehicle(c *gin.Context) {
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

	record, err := h.service.Return(c.Request.Context(), domain.ReturnRequest{
		RecordID:        req.RecordID,
		VehicleID:       req.VehicleID,
		Mileage:         req.Mileage,
		FuelConsumption: req.FuelConsumption,
		RoadSectionIDs:  req.RoadSectionIDs,
	})
	if err != nil {
		writeVehicleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "收车成功",
		"record":  vehicleRecordResponse(record),
	})
}

func writeVehicleError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "车辆操作失败"
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status = http.StatusRequestTimeout
		message = "请求已取消"
	case errors.Is(err, domain.ErrValidation):
		status = http.StatusBadRequest
		message = "参数错误"
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		message = "车辆或出车记录不存在"
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		message = "车辆状态冲突"
	case errors.Is(err, domain.ErrStorage):
		status = http.StatusInternalServerError
	}
	c.JSON(status, gin.H{"error": message})
}

func vehicleRecordResponse(record domain.Record) models.VehicleRecord {
	return models.VehicleRecord{
		ID:              record.ID,
		VehicleID:       record.VehicleID,
		DriverID:        record.DriverID,
		DepartTime:      record.DepartTime,
		ReturnTime:      record.ReturnTime,
		Mileage:         record.Mileage,
		FuelConsumption: record.FuelConsumption,
		RoadSectionIDs:  record.RoadSectionIDs,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func GetMaintenanceReminders(c *gin.Context) {
	var vehicles []models.Vehicle
	database.DB.WithContext(c.Request.Context()).Find(&vehicles)

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

func (h *VehicleHandler) UpdateVehicleStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	vehicle, err := h.service.UpdateStatus(c.Request.Context(), domain.UpdateStatusRequest{
		VehicleID: uint(id),
		Status:    req.Status,
	})
	if err != nil {
		writeVehicleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "状态更新成功",
		"vehicle": vehicleResponse(vehicle),
	})
}

func vehicleResponse(vehicle domain.Vehicle) models.Vehicle {
	return models.Vehicle{
		ID:                     vehicle.ID,
		PlateNumber:            vehicle.PlateNumber,
		Type:                   vehicle.Type,
		Status:                 vehicle.Status,
		Mileage:                vehicle.Mileage,
		LastMaintenance:        vehicle.LastMaintenance,
		LastMaintenanceMileage: vehicle.LastMaintenanceMileage,
		CreatedAt:              vehicle.CreatedAt,
		UpdatedAt:              vehicle.UpdatedAt,
	}
}
