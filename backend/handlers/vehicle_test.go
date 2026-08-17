package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"sanitation-backend/handlers"
	"sanitation-backend/internal/vehicle/domain"
	"sanitation-backend/internal/vehicle/repository"
	"sanitation-backend/internal/vehicle/service"

	"github.com/gin-gonic/gin"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

func TestVehicleLifecycleUpdatesTripAndMileage(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{
		ID:      1,
		Status:  domain.StatusAvailable,
		Mileage: 100,
	})
	router := newVehicleRouter(repo)

	start := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1,"driver_id":7}`)
	assertHTTPStatus(t, start, http.StatusOK)
	var startResponse vehicleResponse
	decodeResponse(t, start, &startResponse)
	if startResponse.Record.ID == 0 || startResponse.Record.VehicleID != 1 {
		t.Fatalf("unexpected start record: %+v", startResponse.Record)
	}
	if startResponse.Record.DepartTime == nil || startResponse.Record.CreatedAt.IsZero() || startResponse.Record.UpdatedAt.IsZero() {
		t.Fatalf("start response lost record timestamps: %+v", startResponse.Record)
	}

	afterStart := repo.Snapshot()
	if got := afterStart.Vehicles[1].Status; got != domain.StatusOnDuty {
		t.Fatalf("vehicle status after start = %q, want %q", got, domain.StatusOnDuty)
	}
	if len(afterStart.Records) != 1 {
		t.Fatalf("record count after start = %d, want 1", len(afterStart.Records))
	}

	returnBody := `{"record_id":1,"mileage":12.5,"fuel_consumption":3.25,"road_section_ids":"2,3"}`
	returned := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/return", returnBody)
	assertHTTPStatus(t, returned, http.StatusOK)
	var returnResponse vehicleResponse
	decodeResponse(t, returned, &returnResponse)
	if returnResponse.Record.ReturnTime == nil {
		t.Fatal("return response did not close the active record")
	}

	afterReturn := repo.Snapshot()
	vehicle := afterReturn.Vehicles[1]
	if vehicle.Status != domain.StatusAvailable {
		t.Fatalf("vehicle status after return = %q, want %q", vehicle.Status, domain.StatusAvailable)
	}
	if vehicle.Mileage != 112.5 {
		t.Fatalf("vehicle mileage after return = %.1f, want 112.5", vehicle.Mileage)
	}
	record := afterReturn.Records[1]
	if record.ReturnTime == nil || record.Mileage != 12.5 || record.FuelConsumption != 3.25 || record.RoadSectionIDs != "2,3" {
		t.Fatalf("unexpected closed record: %+v", record)
	}
}

func TestVehicleCannotStartTwice(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable})
	router := newVehicleRouter(repo)

	first := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`)
	assertHTTPStatus(t, first, http.StatusOK)
	second := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`)
	assertHTTPError(t, second, http.StatusConflict, "车辆状态冲突")

	if got := len(repo.Snapshot().Records); got != 1 {
		t.Fatalf("record count = %d, want 1", got)
	}
}

func TestVehicleCannotReturnTwice(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable, Mileage: 50})
	router := newVehicleRouter(repo)

	assertHTTPStatus(t, performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`), http.StatusOK)
	returnBody := `{"record_id":1,"mileage":8}`
	assertHTTPStatus(t, performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/return", returnBody), http.StatusOK)
	second := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/return", returnBody)
	assertHTTPError(t, second, http.StatusConflict, "车辆状态冲突")

	if got := repo.Snapshot().Vehicles[1].Mileage; got != 58 {
		t.Fatalf("vehicle mileage after duplicate return = %.1f, want 58", got)
	}
}

func TestConcurrentVehicleStartAllowsOneActiveTrip(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable})
	router := newVehicleRouter(repo)

	const callers = 12
	ready := make(chan struct{})
	statuses := make(chan int, callers)
	var waitGroup sync.WaitGroup
	for i := 0; i < callers; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-ready
			response := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`)
			statuses <- response.Code
		}()
	}
	close(ready)
	waitGroup.Wait()
	close(statuses)

	successes := 0
	conflicts := 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent response status: %d", status)
		}
	}
	if successes != 1 || conflicts != callers-1 {
		t.Fatalf("successes/conflicts = %d/%d, want 1/%d", successes, conflicts, callers-1)
	}
	if got := len(repo.Snapshot().Records); got != 1 {
		t.Fatalf("active record count = %d, want 1", got)
	}
}

func TestVehicleStatusUpdateRejectsActiveTrip(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable})
	router := newVehicleRouter(repo)

	assertHTTPStatus(t, performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`), http.StatusOK)
	response := performJSONRequest(router, context.Background(), http.MethodPut, "/api/vehicles/1/status", `{"status":"maintenance"}`)
	assertHTTPError(t, response, http.StatusConflict, "车辆状态冲突")

	snapshot := repo.Snapshot()
	if snapshot.Vehicles[1].Status != domain.StatusOnDuty || len(snapshot.Records) != 1 {
		t.Fatalf("status update broke active trip invariant: %+v", snapshot)
	}
}

func TestConcurrentVehicleStartAndStatusUpdatePreserveInvariant(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable})
	router := newVehicleRouter(repo)

	ready := make(chan struct{})
	statuses := make(chan int, 2)
	var waitGroup sync.WaitGroup
	requests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/vehicles/start", body: `{"vehicle_id":1}`},
		{method: http.MethodPut, path: "/api/vehicles/1/status", body: `{"status":"maintenance"}`},
	}
	for _, request := range requests {
		request := request
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-ready
			response := performJSONRequest(router, context.Background(), request.method, request.path, request.body)
			statuses <- response.Code
		}()
	}
	close(ready)
	waitGroup.Wait()
	close(statuses)

	successes := 0
	conflicts := 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected response status: %d", status)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes/conflicts = %d/%d, want 1/1", successes, conflicts)
	}

	snapshot := repo.Snapshot()
	vehicle := snapshot.Vehicles[1]
	activeRecords := 0
	for _, record := range snapshot.Records {
		if record.ReturnTime == nil {
			activeRecords++
		}
	}
	if vehicle.Status == domain.StatusOnDuty && activeRecords != 1 {
		t.Fatalf("on-duty vehicle has %d active records", activeRecords)
	}
	if vehicle.Status == domain.StatusMaintenance && activeRecords != 0 {
		t.Fatalf("maintenance vehicle has %d active records", activeRecords)
	}
	if vehicle.Status != domain.StatusOnDuty && vehicle.Status != domain.StatusMaintenance {
		t.Fatalf("unexpected final vehicle state: %+v", vehicle)
	}
}

func TestVehicleOperationFailureLeavesStateUnchanged(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable, Mileage: 20})
		repo.FailNext(repository.FailAfterRecordMutation)
		router := newVehicleRouter(repo)

		response := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`)
		assertHTTPError(t, response, http.StatusInternalServerError, "车辆操作失败")
		snapshot := repo.Snapshot()
		if snapshot.Vehicles[1].Status != domain.StatusAvailable || len(snapshot.Records) != 0 {
			t.Fatalf("state changed after failed start: %+v", snapshot)
		}
	})

	t.Run("return", func(t *testing.T) {
		repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable, Mileage: 20})
		router := newVehicleRouter(repo)
		assertHTTPStatus(t, performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`), http.StatusOK)
		before := repo.Snapshot()
		repo.FailNext(repository.FailAfterRecordMutation)

		response := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/return", `{"record_id":1,"mileage":10}`)
		assertHTTPError(t, response, http.StatusInternalServerError, "车辆操作失败")
		after := repo.Snapshot()
		if after.Vehicles[1] != before.Vehicles[1] || after.Records[1].ReturnTime != nil {
			t.Fatalf("state changed after failed return: before=%+v after=%+v", before, after)
		}
	})
}

func TestCanceledVehicleRequestLeavesStateUnchanged(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable, Mileage: 20})
	router := newVehicleRouter(repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	response := performJSONRequest(router, ctx, http.MethodPost, "/api/vehicles/start", `{"vehicle_id":1}`)
	assertHTTPError(t, response, http.StatusRequestTimeout, "请求已取消")
	snapshot := repo.Snapshot()
	if snapshot.Vehicles[1].Status != domain.StatusAvailable || len(snapshot.Records) != 0 {
		t.Fatalf("state changed for canceled request: %+v", snapshot)
	}
}

func TestVehicleHTTPErrorMapping(t *testing.T) {
	repo := repository.NewMemory(repository.MemoryVehicle{ID: 1, Status: domain.StatusAvailable})
	router := newVehicleRouter(repo)

	validation := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/return", `{}`)
	assertHTTPError(t, validation, http.StatusBadRequest, "参数错误")
	notFound := performJSONRequest(router, context.Background(), http.MethodPost, "/api/vehicles/start", `{"vehicle_id":99}`)
	assertHTTPError(t, notFound, http.StatusNotFound, "车辆或出车记录不存在")
}

type vehicleResponse struct {
	Message string `json:"message"`
	Record  struct {
		ID              uint       `json:"id"`
		VehicleID       uint       `json:"vehicle_id"`
		DepartTime      *time.Time `json:"depart_time"`
		ReturnTime      *time.Time `json:"return_time"`
		Mileage         float64    `json:"mileage"`
		FuelConsumption float64    `json:"fuel_consumption"`
		RoadSectionIDs  string     `json:"road_section_ids"`
		CreatedAt       time.Time  `json:"created_at"`
		UpdatedAt       time.Time  `json:"updated_at"`
	} `json:"record"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func newVehicleRouter(repo *repository.Memory) *gin.Engine {
	gin.SetMode(gin.TestMode)
	vehicleService := service.New(repo, fixedClock{now: time.Date(2026, time.August, 17, 8, 30, 0, 0, time.UTC)})
	vehicleHandler := handlers.NewVehicleHandler(vehicleService)
	router := gin.New()
	router.POST("/api/vehicles/start", vehicleHandler.StartVehicle)
	router.POST("/api/vehicles/return", vehicleHandler.ReturnVehicle)
	router.PUT("/api/vehicles/:id/status", vehicleHandler.UpdateVehicleStatus)
	return router
}

func performJSONRequest(router http.Handler, ctx context.Context, method, path, body string) *httptest.ResponseRecorder {
	request, err := http.NewRequestWithContext(ctx, method, path, bytes.NewBufferString(body))
	if err != nil {
		panic(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func assertHTTPStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("HTTP status = %d, want %d; body=%s", response.Code, want, response.Body.String())
	}
}

func assertHTTPError(t *testing.T, response *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	assertHTTPStatus(t, response, status)
	var body errorResponse
	decodeResponse(t, response, &body)
	if body.Error != message {
		t.Fatalf("error message = %q, want %q", body.Error, message)
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}
