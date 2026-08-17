# Sanitation Operations Platform

This repository contains a Vue frontend and a Go backend for sanitation work
plans, inspections, vehicles, complaints, and monthly assessments.

## Vehicle lifecycle baseline

Vehicle dispatch follows these public invariants:

- An available vehicle can start one active trip.
- A vehicle cannot have more than one trip without a return time.
- Starting a trip creates the record and changes the vehicle to `on_duty` in
  one transaction.
- Returning a trip closes the record, adds its mileage, and changes the
  vehicle to `available` in one transaction.
- Repeated or conflicting state transitions return HTTP `409`.
- Canceled requests do not commit state changes.

The backend separates this behavior into:

- `internal/vehicle/domain`: commands, records, states, and error categories.
- `internal/vehicle/service`: validation, clock injection, and orchestration.
- `internal/vehicle/repository`: PostgreSQL/GORM transactions and an in-memory
  repository for deterministic tests.
- `handlers`: the existing Gin API contract and stable HTTP error mapping.

The existing endpoints remain available:

```text
POST /api/vehicles/start
POST /api/vehicles/return
```

## Requirements

- Go 1.21 or newer
- Node.js 20 or newer
- Docker with Compose for the full PostgreSQL-backed application

## Backend verification

Run from `backend`:

```bash
go test ./... -count=1
go test -race ./handlers -run TestConcurrentVehicleStartAllowsOneActiveTrip -count=1
go build ./...
go vet ./...
```

The tests use the real Gin handlers and vehicle service with a deterministic
in-memory repository. They do not require PostgreSQL or network access.

The production repository has an opt-in PostgreSQL integration suite. It
creates and removes a uniquely named schema, leaving existing application
tables and data unchanged.

```bash
docker compose up -d postgres
cd backend
VEHICLE_TEST_DATABASE_URL='host=localhost port=5432 user=sanitation password=sanit@2024 dbname=sanitation sslmode=disable' \
  go test -race -tags=integration ./internal/vehicle/repository -count=1
```

PowerShell equivalent:

```powershell
docker compose up -d postgres
Set-Location backend
$env:VEHICLE_TEST_DATABASE_URL='host=localhost port=5432 user=sanitation password=sanit@2024 dbname=sanitation sslmode=disable'
go test -race -tags=integration ./internal/vehicle/repository -count=1
```

Startup also checks for legacy vehicles with more than one active trip before
creating the uniqueness constraint. If any are found, migration stops and
reports the affected vehicle IDs; it never deletes or rewrites those records.

## Frontend verification

Run from `frontend`:

```bash
npm ci
npm run build
```

## Run the full application

From the repository root:

```bash
docker compose up --build
```

The frontend listens on `http://localhost:5173`; the backend listens on
`http://localhost:8653`.
