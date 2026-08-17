package service

import (
	"context"
	"fmt"
	"time"

	"sanitation-backend/internal/vehicle/domain"
)

type Clock interface {
	Now() time.Time
}

type Repository interface {
	Start(ctx context.Context, request domain.StartRequest, now time.Time) (domain.Record, error)
	Return(ctx context.Context, request domain.ReturnRequest, now time.Time) (domain.Record, error)
	UpdateStatus(ctx context.Context, request domain.UpdateStatusRequest, now time.Time) (domain.Vehicle, error)
}

type Service struct {
	repository Repository
	clock      Clock
}

func New(repository Repository, clock Clock) *Service {
	return &Service{repository: repository, clock: clock}
}

func (s *Service) Start(ctx context.Context, request domain.StartRequest) (domain.Record, error) {
	if request.VehicleID == 0 {
		return domain.Record{}, fmt.Errorf("%w: vehicle_id is required", domain.ErrValidation)
	}
	if err := ctx.Err(); err != nil {
		return domain.Record{}, err
	}
	return s.repository.Start(ctx, request, s.clock.Now())
}

func (s *Service) Return(ctx context.Context, request domain.ReturnRequest) (domain.Record, error) {
	if (request.RecordID == nil) == (request.VehicleID == nil) {
		return domain.Record{}, fmt.Errorf("%w: provide exactly one of record_id or vehicle_id", domain.ErrValidation)
	}
	if request.RecordID != nil && *request.RecordID == 0 {
		return domain.Record{}, fmt.Errorf("%w: record_id must be positive", domain.ErrValidation)
	}
	if request.VehicleID != nil && *request.VehicleID == 0 {
		return domain.Record{}, fmt.Errorf("%w: vehicle_id must be positive", domain.ErrValidation)
	}
	if request.Mileage < 0 {
		return domain.Record{}, fmt.Errorf("%w: mileage must not be negative", domain.ErrValidation)
	}
	if request.FuelConsumption < 0 {
		return domain.Record{}, fmt.Errorf("%w: fuel_consumption must not be negative", domain.ErrValidation)
	}
	if err := ctx.Err(); err != nil {
		return domain.Record{}, err
	}
	return s.repository.Return(ctx, request, s.clock.Now())
}

func (s *Service) UpdateStatus(ctx context.Context, request domain.UpdateStatusRequest) (domain.Vehicle, error) {
	if request.VehicleID == 0 {
		return domain.Vehicle{}, fmt.Errorf("%w: vehicle_id is required", domain.ErrValidation)
	}
	if request.Status != domain.StatusAvailable && request.Status != domain.StatusMaintenance {
		return domain.Vehicle{}, fmt.Errorf("%w: unsupported vehicle status", domain.ErrValidation)
	}
	if err := ctx.Err(); err != nil {
		return domain.Vehicle{}, err
	}
	return s.repository.UpdateStatus(ctx, request, s.clock.Now())
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now()
}
