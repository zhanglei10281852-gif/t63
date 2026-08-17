//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"sanitation-backend/internal/vehicle/domain"
	"sanitation-backend/internal/vehicle/repository"
	"sanitation-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGORMVehicleLifecyclePersistsResponseFields(t *testing.T) {
	db := openPostgresFixture(t)
	vehicle := createVehicle(t, db, "TEST-LIFECYCLE")
	vehicleRepository := repository.NewGORM(db)
	departTime := time.Date(2026, time.August, 17, 8, 0, 0, 0, time.UTC)

	started, err := vehicleRepository.Start(context.Background(), domain.StartRequest{VehicleID: vehicle.ID}, departTime)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.DepartTime == nil || !started.DepartTime.Equal(departTime) {
		t.Fatalf("Start() depart time = %v, want %v", started.DepartTime, departTime)
	}
	if started.CreatedAt.IsZero() || started.UpdatedAt.IsZero() {
		t.Fatalf("Start() lost persisted timestamps: %+v", started)
	}

	returnTime := departTime.Add(2 * time.Hour)
	returned, err := vehicleRepository.Return(context.Background(), domain.ReturnRequest{
		RecordID: &started.ID,
		Mileage:  24.5,
	}, returnTime)
	if err != nil {
		t.Fatalf("Return() error = %v", err)
	}
	if returned.ReturnTime == nil || !returned.ReturnTime.Equal(returnTime) {
		t.Fatalf("Return() return time = %v, want %v", returned.ReturnTime, returnTime)
	}
	if returned.CreatedAt.IsZero() || returned.UpdatedAt.IsZero() {
		t.Fatalf("Return() lost persisted timestamps: %+v", returned)
	}

	legacyVehicle := createVehicle(t, db, "TEST-LEGACY-01")
	if err := db.Model(&legacyVehicle).Update("status", domain.StatusOnDuty).Error; err != nil {
		t.Fatalf("mark legacy vehicle on duty: %v", err)
	}
	legacyRecord := models.VehicleRecord{VehicleID: legacyVehicle.ID}
	if err := db.Create(&legacyRecord).Error; err != nil {
		t.Fatalf("create legacy record: %v", err)
	}
	legacyReturned, err := vehicleRepository.Return(context.Background(), domain.ReturnRequest{
		RecordID: &legacyRecord.ID,
	}, returnTime)
	if err != nil {
		t.Fatalf("Return() legacy record error = %v", err)
	}
	if legacyReturned.DepartTime != nil {
		t.Fatalf("Return() fabricated legacy depart time: %v", legacyReturned.DepartTime)
	}
}

func TestGORMConcurrentStartAndStatusUpdatePreserveInvariant(t *testing.T) {
	db := openPostgresFixture(t)
	vehicle := createVehicle(t, db, "TEST-CONCURRENCY")
	vehicleRepository := repository.NewGORM(db)

	ready := make(chan struct{})
	errorsByOperation := make(chan error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-ready
		_, err := vehicleRepository.Start(context.Background(), domain.StartRequest{VehicleID: vehicle.ID}, time.Now())
		errorsByOperation <- err
	}()
	go func() {
		defer waitGroup.Done()
		<-ready
		_, err := vehicleRepository.UpdateStatus(context.Background(), domain.UpdateStatusRequest{
			VehicleID: vehicle.ID,
			Status:    domain.StatusMaintenance,
		}, time.Now())
		errorsByOperation <- err
	}()
	close(ready)
	waitGroup.Wait()
	close(errorsByOperation)

	successes := 0
	conflicts := 0
	for err := range errorsByOperation {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent operation error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes/conflicts = %d/%d, want 1/1", successes, conflicts)
	}

	var stored models.Vehicle
	if err := db.First(&stored, vehicle.ID).Error; err != nil {
		t.Fatalf("load vehicle: %v", err)
	}
	var activeRecords int64
	if err := db.Model(&models.VehicleRecord{}).
		Where("vehicle_id = ? AND return_time IS NULL", vehicle.ID).
		Count(&activeRecords).Error; err != nil {
		t.Fatalf("count active records: %v", err)
	}
	switch stored.Status {
	case domain.StatusOnDuty:
		if activeRecords != 1 {
			t.Fatalf("on-duty vehicle has %d active records", activeRecords)
		}
	case domain.StatusMaintenance:
		if activeRecords != 0 {
			t.Fatalf("maintenance vehicle has %d active records", activeRecords)
		}
	default:
		t.Fatalf("unexpected final vehicle state: %+v", stored)
	}
}

func TestAutoMigrateReportsDuplicateActiveRecordsWithoutDeletingThem(t *testing.T) {
	db := openPostgresSchema(t)
	if err := db.AutoMigrate(&models.Vehicle{}, &models.VehicleRecord{}); err != nil {
		t.Fatalf("create legacy tables: %v", err)
	}
	vehicle := createVehicle(t, db, "TEST-DUP-01")
	now := time.Now()
	records := []models.VehicleRecord{
		{VehicleID: vehicle.ID, DepartTime: &now},
		{VehicleID: vehicle.ID, DepartTime: &now},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create duplicate active records: %v", err)
	}

	err := models.AutoMigrate(db)
	if err == nil {
		t.Fatal("AutoMigrate() succeeded with duplicate active records")
	}
	wantVehicleID := fmt.Sprintf("vehicle IDs [%d]", vehicle.ID)
	if !strings.Contains(err.Error(), wantVehicleID) {
		t.Fatalf("AutoMigrate() error = %q, want %q", err, wantVehicleID)
	}

	var count int64
	if err := db.Model(&models.VehicleRecord{}).Where("vehicle_id = ?", vehicle.ID).Count(&count).Error; err != nil {
		t.Fatalf("count preserved records: %v", err)
	}
	if count != 2 {
		t.Fatalf("duplicate record count = %d, want 2", count)
	}
}

func openPostgresFixture(t *testing.T) *gorm.DB {
	t.Helper()
	testDB := openPostgresSchema(t)
	if err := models.AutoMigrate(testDB); err != nil {
		t.Fatalf("migrate integration schema: %v", err)
	}
	return testDB
}

func openPostgresSchema(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("VEHICLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set VEHICLE_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	adminConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse VEHICLE_TEST_DATABASE_URL: %v", err)
	}
	adminSQL := stdlib.OpenDB(*adminConfig)
	adminDB := openGORM(t, adminSQL)
	schema := fmt.Sprintf("vehicle_it_%d", time.Now().UnixNano())
	if err := adminDB.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error; err != nil {
		_ = adminSQL.Close()
		t.Fatalf("create integration schema: %v", err)
	}

	testConfig := adminConfig.Copy()
	if testConfig.RuntimeParams == nil {
		testConfig.RuntimeParams = make(map[string]string)
	}
	testConfig.RuntimeParams["search_path"] = schema
	testSQL := stdlib.OpenDB(*testConfig)
	testDB := openGORM(t, testSQL)
	t.Cleanup(func() {
		if err := testSQL.Close(); err != nil {
			t.Errorf("close integration database: %v", err)
		}
		if err := adminDB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema)).Error; err != nil {
			t.Errorf("drop integration schema: %v", err)
		}
		if err := adminSQL.Close(); err != nil {
			t.Errorf("close admin database: %v", err)
		}
	})

	return testDB
}

func openGORM(t *testing.T, connection *sql.DB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open GORM database: %v", err)
	}
	return db
}

func createVehicle(t *testing.T, db *gorm.DB, plateNumber string) models.Vehicle {
	t.Helper()
	vehicle := models.Vehicle{
		PlateNumber: plateNumber,
		Type:        "sweeper",
		Status:      domain.StatusAvailable,
		Mileage:     100,
	}
	if err := db.Create(&vehicle).Error; err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	return vehicle
}
