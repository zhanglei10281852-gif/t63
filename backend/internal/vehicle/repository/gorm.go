package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sanitation-backend/internal/vehicle/domain"
	"sanitation-backend/models"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORM struct {
	db *gorm.DB
}

func NewGORM(db *gorm.DB) *GORM {
	return &GORM{db: db}
}

func (r *GORM) Start(ctx context.Context, request domain.StartRequest, now time.Time) (domain.Record, error) {
	var result domain.Record
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var vehicle models.Vehicle
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&vehicle, request.VehicleID).Error; err != nil {
			return mapDatabaseError(err, "vehicle")
		}
		if vehicle.Status != domain.StatusAvailable {
			return fmt.Errorf("%w: vehicle is not available", domain.ErrConflict)
		}

		var activeCount int64
		if err := tx.Model(&models.VehicleRecord{}).
			Where("vehicle_id = ? AND return_time IS NULL", vehicle.ID).
			Count(&activeCount).Error; err != nil {
			return mapDatabaseError(err, "active vehicle record")
		}
		if activeCount > 0 {
			return fmt.Errorf("%w: vehicle already has an active record", domain.ErrConflict)
		}

		departTime := now
		record := models.VehicleRecord{
			VehicleID:  vehicle.ID,
			DriverID:   request.DriverID,
			DepartTime: &departTime,
		}
		if err := tx.Create(&record).Error; err != nil {
			return mapDatabaseError(err, "create vehicle record")
		}

		update := tx.Model(&models.Vehicle{}).
			Where("id = ? AND status = ?", vehicle.ID, domain.StatusAvailable).
			Update("status", domain.StatusOnDuty)
		if update.Error != nil {
			return mapDatabaseError(update.Error, "mark vehicle on duty")
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("%w: vehicle state changed", domain.ErrConflict)
		}

		result = recordFromModel(record)
		return nil
	})
	if err != nil {
		return domain.Record{}, err
	}
	return result, nil
}

func (r *GORM) Return(ctx context.Context, request domain.ReturnRequest, now time.Time) (domain.Record, error) {
	var result domain.Record
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record models.VehicleRecord
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"})
		if request.RecordID != nil {
			query = query.Where("id = ?", *request.RecordID)
		} else {
			query = query.Where("vehicle_id = ? AND return_time IS NULL", *request.VehicleID).
				Order("depart_time DESC")
		}
		if err := query.First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) && request.VehicleID != nil {
				var vehicle models.Vehicle
				if vehicleErr := tx.First(&vehicle, *request.VehicleID).Error; vehicleErr != nil {
					return mapDatabaseError(vehicleErr, "vehicle")
				}
				return fmt.Errorf("%w: vehicle has no active record", domain.ErrConflict)
			}
			return mapDatabaseError(err, "vehicle record")
		}
		if record.ReturnTime != nil {
			return fmt.Errorf("%w: vehicle record is already closed", domain.ErrConflict)
		}

		var vehicle models.Vehicle
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&vehicle, record.VehicleID).Error; err != nil {
			return mapDatabaseError(err, "vehicle")
		}
		if vehicle.Status != domain.StatusOnDuty {
			return fmt.Errorf("%w: vehicle is not on duty", domain.ErrConflict)
		}

		updateRecord := tx.Model(&models.VehicleRecord{}).
			Where("id = ? AND return_time IS NULL", record.ID).
			Updates(map[string]interface{}{
				"return_time":      now,
				"mileage":          request.Mileage,
				"fuel_consumption": request.FuelConsumption,
				"road_section_ids": request.RoadSectionIDs,
			})
		if updateRecord.Error != nil {
			return mapDatabaseError(updateRecord.Error, "close vehicle record")
		}
		if updateRecord.RowsAffected != 1 {
			return fmt.Errorf("%w: vehicle record state changed", domain.ErrConflict)
		}

		newMileage := vehicle.Mileage + request.Mileage
		updateVehicle := tx.Model(&models.Vehicle{}).
			Where("id = ? AND status = ?", vehicle.ID, domain.StatusOnDuty).
			Updates(map[string]interface{}{
				"status":  domain.StatusAvailable,
				"mileage": newMileage,
			})
		if updateVehicle.Error != nil {
			return mapDatabaseError(updateVehicle.Error, "mark vehicle available")
		}
		if updateVehicle.RowsAffected != 1 {
			return fmt.Errorf("%w: vehicle state changed", domain.ErrConflict)
		}

		if err := tx.First(&record, record.ID).Error; err != nil {
			return mapDatabaseError(err, "reload vehicle record")
		}
		result = recordFromModel(record)
		return nil
	})
	if err != nil {
		return domain.Record{}, err
	}
	return result, nil
}

func (r *GORM) UpdateStatus(ctx context.Context, request domain.UpdateStatusRequest, now time.Time) (domain.Vehicle, error) {
	var result domain.Vehicle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var vehicle models.Vehicle
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&vehicle, request.VehicleID).Error; err != nil {
			return mapDatabaseError(err, "vehicle")
		}
		if vehicle.Status == domain.StatusOnDuty {
			return fmt.Errorf("%w: active vehicle must be returned first", domain.ErrConflict)
		}

		var activeCount int64
		if err := tx.Model(&models.VehicleRecord{}).
			Where("vehicle_id = ? AND return_time IS NULL", vehicle.ID).
			Count(&activeCount).Error; err != nil {
			return mapDatabaseError(err, "active vehicle record")
		}
		if activeCount > 0 {
			return fmt.Errorf("%w: vehicle has an active record", domain.ErrConflict)
		}

		updates := map[string]interface{}{"status": request.Status}
		if request.Status == domain.StatusAvailable && vehicle.Status == domain.StatusMaintenance {
			updates["last_maintenance"] = now
			updates["last_maintenance_mileage"] = vehicle.Mileage
		}
		update := tx.Model(&models.Vehicle{}).
			Where("id = ? AND status = ?", vehicle.ID, vehicle.Status).
			Updates(updates)
		if update.Error != nil {
			return mapDatabaseError(update.Error, "update vehicle status")
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("%w: vehicle state changed", domain.ErrConflict)
		}
		if err := tx.First(&vehicle, vehicle.ID).Error; err != nil {
			return mapDatabaseError(err, "reload vehicle")
		}
		result = vehicleFromModel(vehicle)
		return nil
	})
	if err != nil {
		return domain.Vehicle{}, err
	}
	return result, nil
}

func recordFromModel(record models.VehicleRecord) domain.Record {
	return domain.Record{
		ID:              record.ID,
		VehicleID:       record.VehicleID,
		DriverID:        cloneUint(record.DriverID),
		DepartTime:      cloneTime(record.DepartTime),
		ReturnTime:      cloneTime(record.ReturnTime),
		Mileage:         record.Mileage,
		FuelConsumption: record.FuelConsumption,
		RoadSectionIDs:  record.RoadSectionIDs,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func vehicleFromModel(vehicle models.Vehicle) domain.Vehicle {
	return domain.Vehicle{
		ID:                     vehicle.ID,
		PlateNumber:            vehicle.PlateNumber,
		Type:                   vehicle.Type,
		Status:                 vehicle.Status,
		Mileage:                vehicle.Mileage,
		LastMaintenance:        cloneTime(vehicle.LastMaintenance),
		LastMaintenanceMileage: vehicle.LastMaintenanceMileage,
		CreatedAt:              vehicle.CreatedAt,
		UpdatedAt:              vehicle.UpdatedAt,
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func mapDatabaseError(err error, operation string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, operation)
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return fmt.Errorf("%w: %s", domain.ErrConflict, operation)
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return fmt.Errorf("%w: %s", domain.ErrConflict, operation)
	}
	return fmt.Errorf("%w: %s: %v", domain.ErrStorage, operation, err)
}
