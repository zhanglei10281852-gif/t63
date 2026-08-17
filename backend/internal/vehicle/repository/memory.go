package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sanitation-backend/internal/vehicle/domain"
)

type FailurePoint string

const (
	FailNone                FailurePoint = ""
	FailAfterRecordMutation FailurePoint = "after_record_mutation"
)

type MemoryVehicle struct {
	ID                     uint
	PlateNumber            string
	Type                   string
	Status                 string
	Mileage                float64
	LastMaintenance        *time.Time
	LastMaintenanceMileage float64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type MemorySnapshot struct {
	Vehicles map[uint]MemoryVehicle
	Records  map[uint]domain.Record
}

type Memory struct {
	mu         sync.Mutex
	vehicles   map[uint]MemoryVehicle
	records    map[uint]domain.Record
	nextRecord uint
	failNextAt FailurePoint
}

func NewMemory(vehicles ...MemoryVehicle) *Memory {
	repository := &Memory{
		vehicles:   make(map[uint]MemoryVehicle, len(vehicles)),
		records:    make(map[uint]domain.Record),
		nextRecord: 1,
	}
	for _, vehicle := range vehicles {
		repository.vehicles[vehicle.ID] = vehicle
	}
	return repository
}

func (r *Memory) FailNext(point FailurePoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failNextAt = point
}

func (r *Memory) Snapshot() MemorySnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return MemorySnapshot{
		Vehicles: cloneVehicles(r.vehicles),
		Records:  cloneRecords(r.records),
	}
}

func (r *Memory) Start(ctx context.Context, request domain.StartRequest, now time.Time) (domain.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Record{}, err
	}

	vehicle, ok := r.vehicles[request.VehicleID]
	if !ok {
		return domain.Record{}, fmt.Errorf("%w: vehicle", domain.ErrNotFound)
	}
	if vehicle.Status != domain.StatusAvailable {
		return domain.Record{}, fmt.Errorf("%w: vehicle is not available", domain.ErrConflict)
	}
	for _, record := range r.records {
		if record.VehicleID == vehicle.ID && record.ReturnTime == nil {
			return domain.Record{}, fmt.Errorf("%w: vehicle already has an active record", domain.ErrConflict)
		}
	}

	vehicles := cloneVehicles(r.vehicles)
	records := cloneRecords(r.records)
	departTime := now
	record := domain.Record{
		ID:         r.nextRecord,
		VehicleID:  vehicle.ID,
		DriverID:   cloneUint(request.DriverID),
		DepartTime: &departTime,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	records[record.ID] = record
	if r.consumeFailure(FailAfterRecordMutation) {
		return domain.Record{}, fmt.Errorf("%w: simulated vehicle update failure", domain.ErrStorage)
	}
	vehicle.Status = domain.StatusOnDuty
	vehicles[vehicle.ID] = vehicle
	if err := ctx.Err(); err != nil {
		return domain.Record{}, err
	}

	r.vehicles = vehicles
	r.records = records
	r.nextRecord++
	return cloneRecord(record), nil
}

func (r *Memory) Return(ctx context.Context, request domain.ReturnRequest, now time.Time) (domain.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Record{}, err
	}

	record, err := r.findRecord(request)
	if err != nil {
		return domain.Record{}, err
	}
	if record.ReturnTime != nil {
		return domain.Record{}, fmt.Errorf("%w: vehicle record is already closed", domain.ErrConflict)
	}
	vehicle, ok := r.vehicles[record.VehicleID]
	if !ok {
		return domain.Record{}, fmt.Errorf("%w: vehicle", domain.ErrNotFound)
	}
	if vehicle.Status != domain.StatusOnDuty {
		return domain.Record{}, fmt.Errorf("%w: vehicle is not on duty", domain.ErrConflict)
	}

	vehicles := cloneVehicles(r.vehicles)
	records := cloneRecords(r.records)
	returnTime := now
	record.ReturnTime = &returnTime
	record.Mileage = request.Mileage
	record.FuelConsumption = request.FuelConsumption
	record.RoadSectionIDs = request.RoadSectionIDs
	record.UpdatedAt = now
	records[record.ID] = record
	if r.consumeFailure(FailAfterRecordMutation) {
		return domain.Record{}, fmt.Errorf("%w: simulated vehicle update failure", domain.ErrStorage)
	}
	vehicle.Status = domain.StatusAvailable
	vehicle.Mileage += request.Mileage
	vehicles[vehicle.ID] = vehicle
	if err := ctx.Err(); err != nil {
		return domain.Record{}, err
	}

	r.vehicles = vehicles
	r.records = records
	return cloneRecord(record), nil
}

func (r *Memory) UpdateStatus(ctx context.Context, request domain.UpdateStatusRequest, now time.Time) (domain.Vehicle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Vehicle{}, err
	}

	vehicle, ok := r.vehicles[request.VehicleID]
	if !ok {
		return domain.Vehicle{}, fmt.Errorf("%w: vehicle", domain.ErrNotFound)
	}
	if vehicle.Status == domain.StatusOnDuty {
		return domain.Vehicle{}, fmt.Errorf("%w: active vehicle must be returned first", domain.ErrConflict)
	}
	for _, record := range r.records {
		if record.VehicleID == vehicle.ID && record.ReturnTime == nil {
			return domain.Vehicle{}, fmt.Errorf("%w: vehicle has an active record", domain.ErrConflict)
		}
	}

	vehicles := cloneVehicles(r.vehicles)
	if request.Status == domain.StatusAvailable && vehicle.Status == domain.StatusMaintenance {
		maintenanceTime := now
		vehicle.LastMaintenance = &maintenanceTime
		vehicle.LastMaintenanceMileage = vehicle.Mileage
	}
	vehicle.Status = request.Status
	vehicle.UpdatedAt = now
	vehicles[vehicle.ID] = vehicle
	if err := ctx.Err(); err != nil {
		return domain.Vehicle{}, err
	}
	r.vehicles = vehicles
	return vehicleFromMemory(vehicle), nil
}

func (r *Memory) findRecord(request domain.ReturnRequest) (domain.Record, error) {
	if request.RecordID != nil {
		record, ok := r.records[*request.RecordID]
		if !ok {
			return domain.Record{}, fmt.Errorf("%w: vehicle record", domain.ErrNotFound)
		}
		return cloneRecord(record), nil
	}
	if _, ok := r.vehicles[*request.VehicleID]; !ok {
		return domain.Record{}, fmt.Errorf("%w: vehicle", domain.ErrNotFound)
	}
	var latest domain.Record
	for _, record := range r.records {
		if record.VehicleID == *request.VehicleID && record.ReturnTime == nil && record.ID > latest.ID {
			latest = record
		}
	}
	if latest.ID == 0 {
		return domain.Record{}, fmt.Errorf("%w: vehicle has no active record", domain.ErrConflict)
	}
	return cloneRecord(latest), nil
}

func (r *Memory) consumeFailure(point FailurePoint) bool {
	if r.failNextAt != point {
		return false
	}
	r.failNextAt = FailNone
	return true
}

func cloneVehicles(source map[uint]MemoryVehicle) map[uint]MemoryVehicle {
	clone := make(map[uint]MemoryVehicle, len(source))
	for id, vehicle := range source {
		vehicle.LastMaintenance = cloneTime(vehicle.LastMaintenance)
		clone[id] = vehicle
	}
	return clone
}

func cloneRecords(source map[uint]domain.Record) map[uint]domain.Record {
	clone := make(map[uint]domain.Record, len(source))
	for id, record := range source {
		clone[id] = cloneRecord(record)
	}
	return clone
}

func cloneRecord(record domain.Record) domain.Record {
	record.DriverID = cloneUint(record.DriverID)
	record.DepartTime = cloneTime(record.DepartTime)
	record.ReturnTime = cloneTime(record.ReturnTime)
	return record
}

func vehicleFromMemory(vehicle MemoryVehicle) domain.Vehicle {
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

func cloneUint(value *uint) *uint {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
