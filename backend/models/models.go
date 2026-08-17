package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"unique;size:50;not null" json:"username"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	RealName  string    `gorm:"size:50" json:"real_name"`
	Role      string    `gorm:"size:20;not null" json:"role"` // admin, area_manager
	AreaID    *uint     `json:"area_id"`
	Area      *Area     `gorm:"foreignKey:AreaID" json:"area,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Area struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RoadSection struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	AreaID    uint      `gorm:"not null" json:"area_id"`
	Area      Area      `gorm:"foreignKey:AreaID" json:"area,omitempty"`
	Level     int       `gorm:"not null" json:"level"` // 1, 2, 3 级
	Length    float64   `json:"length"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Worker struct {
	ID            uint         `gorm:"primaryKey" json:"id"`
	Name          string       `gorm:"size:50;not null" json:"name"`
	Phone         string       `gorm:"size:20" json:"phone"`
	IDCard        string       `gorm:"size:20" json:"id_card"`
	AreaID        uint         `gorm:"not null" json:"area_id"`
	Area          Area         `gorm:"foreignKey:AreaID" json:"area,omitempty"`
	RoadSectionID *uint        `json:"road_section_id"`
	RoadSection   *RoadSection `gorm:"foreignKey:RoadSectionID" json:"road_section,omitempty"`
	Status        string       `gorm:"size:20;default:active" json:"status"` // active, leave, quit
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type Vehicle struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	PlateNumber            string     `gorm:"size:20;unique;not null" json:"plate_number"`
	Type                   string     `gorm:"size:30;not null" json:"type"`            // sprinkler, sweeper, garbage_truck
	Status                 string     `gorm:"size:20;default:available" json:"status"` // available, on_duty, maintenance
	Mileage                float64    `gorm:"default:0" json:"mileage"`
	LastMaintenance        *time.Time `json:"last_maintenance"`
	LastMaintenanceMileage float64    `gorm:"default:0" json:"last_maintenance_mileage"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type VehicleRecord struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	VehicleID       uint       `gorm:"not null" json:"vehicle_id"`
	Vehicle         Vehicle    `gorm:"foreignKey:VehicleID" json:"vehicle,omitempty"`
	DriverID        *uint      `json:"driver_id"`
	Driver          *Worker    `gorm:"foreignKey:DriverID" json:"driver,omitempty"`
	DepartTime      *time.Time `json:"depart_time"`
	ReturnTime      *time.Time `json:"return_time"`
	Mileage         float64    `gorm:"default:0" json:"mileage"`
	FuelConsumption float64    `gorm:"default:0" json:"fuel_consumption"`
	RoadSectionIDs  string     `gorm:"size:255" json:"road_section_ids"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type WorkPlan struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	RoadSectionID uint        `gorm:"not null" json:"road_section_id"`
	RoadSection   RoadSection `gorm:"foreignKey:RoadSectionID" json:"road_section,omitempty"`
	WorkerID      *uint       `json:"worker_id"`
	Worker        *Worker     `gorm:"foreignKey:WorkerID" json:"worker,omitempty"`
	PlanDate      string      `gorm:"size:20;not null" json:"plan_date"`     // YYYY-MM-DD
	PlanTime      string      `gorm:"size:10;not null" json:"plan_time"`     // HH:MM
	Status        string      `gorm:"size:20;default:pending" json:"status"` // pending, completed, late, missed
	WorkMethod    string      `gorm:"size:20" json:"work_method"`            // manual, mechanical, washing
	CheckinTime   *time.Time  `json:"checkin_time"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type QualityInspection struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	RoadSectionID uint        `gorm:"not null" json:"road_section_id"`
	RoadSection   RoadSection `gorm:"foreignKey:RoadSectionID" json:"road_section,omitempty"`
	InspectorID   uint        `gorm:"not null" json:"inspector_id"`
	Inspector     User        `gorm:"foreignKey:InspectorID" json:"inspector,omitempty"`
	InspectTime   time.Time   `json:"inspect_time"`
	Score         int         `gorm:"default:100" json:"score"`
	Deductions    string      `gorm:"type:text" json:"deductions"` // JSON array of deductions
	Remark        string      `gorm:"size:500" json:"remark"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type DeductionItem struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type Complaint struct {
	ID               uint        `gorm:"primaryKey" json:"id"`
	Content          string      `gorm:"type:text;not null" json:"content"`
	RoadSectionID    uint        `gorm:"not null" json:"road_section_id"`
	RoadSection      RoadSection `gorm:"foreignKey:RoadSectionID" json:"road_section,omitempty"`
	ComplaintTime    time.Time   `json:"complaint_time"`
	Complainant      string      `gorm:"size:50" json:"complainant"`
	ComplainantPhone string      `gorm:"size:20" json:"complainant_phone"`
	Status           string      `gorm:"size:20;default:pending" json:"status"` // pending, processing, resolved, invalid
	AssignedTo       *uint       `json:"assigned_to"`
	AssignedUser     *User       `gorm:"foreignKey:AssignedTo" json:"assigned_user,omitempty"`
	HandleResult     string      `gorm:"type:text" json:"handle_result"`
	HandleTime       *time.Time  `json:"handle_time"`
	IsValid          *bool       `json:"is_valid"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

type MonthlyAssessment struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Month              string    `gorm:"size:10;not null" json:"month"`       // YYYY-MM
	TargetType         string    `gorm:"size:20;not null" json:"target_type"` // area, worker
	TargetID           uint      `gorm:"not null" json:"target_id"`
	TargetName         string    `gorm:"size:100" json:"target_name"`
	AreaID             *uint     `json:"area_id"`
	CompletionRate     float64   `gorm:"default:0" json:"completion_rate"`
	QualityScore       float64   `gorm:"default:0" json:"quality_score"`
	ComplaintDeduction int       `gorm:"default:0" json:"complaint_deduction"`
	TotalScore         float64   `gorm:"default:0" json:"total_score"`
	Grade              string    `gorm:"size:20" json:"grade"` // excellent, good, pass, fail
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&User{},
		&Area{},
		&RoadSection{},
		&Worker{},
		&Vehicle{},
		&VehicleRecord{},
		&WorkPlan{},
		&QualityInspection{},
		&Complaint{},
		&MonthlyAssessment{},
	); err != nil {
		return err
	}
	type duplicateActiveRecord struct {
		VehicleID uint `gorm:"column:vehicle_id"`
	}
	var duplicates []duplicateActiveRecord
	if err := db.Raw(`SELECT vehicle_id
		FROM vehicle_records
		WHERE return_time IS NULL
		GROUP BY vehicle_id
		HAVING COUNT(*) > 1
		ORDER BY vehicle_id
		LIMIT 10`).Scan(&duplicates).Error; err != nil {
		return fmt.Errorf("check duplicate active vehicle records: %w", err)
	}
	if len(duplicates) > 0 {
		vehicleIDs := make([]uint, 0, len(duplicates))
		for _, duplicate := range duplicates {
			vehicleIDs = append(vehicleIDs, duplicate.VehicleID)
		}
		return fmt.Errorf("cannot enforce one active vehicle record: duplicate active records exist for vehicle IDs %v", vehicleIDs)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_vehicle_records_one_active
		ON vehicle_records (vehicle_id) WHERE return_time IS NULL`).Error; err != nil {
		return fmt.Errorf("create active vehicle record constraint: %w", err)
	}
	return nil
}
