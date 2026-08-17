package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("vehicle resource not found")
	ErrConflict   = errors.New("vehicle state conflict")
	ErrValidation = errors.New("vehicle request validation failed")
	ErrStorage    = errors.New("vehicle storage operation failed")
)

const (
	StatusAvailable   = "available"
	StatusOnDuty      = "on_duty"
	StatusMaintenance = "maintenance"
)

type StartRequest struct {
	VehicleID uint
	DriverID  *uint
}

type ReturnRequest struct {
	RecordID        *uint
	VehicleID       *uint
	Mileage         float64
	FuelConsumption float64
	RoadSectionIDs  string
}

type UpdateStatusRequest struct {
	VehicleID uint
	Status    string
}

type Record struct {
	ID              uint
	VehicleID       uint
	DriverID        *uint
	DepartTime      *time.Time
	ReturnTime      *time.Time
	Mileage         float64
	FuelConsumption float64
	RoadSectionIDs  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Vehicle struct {
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
