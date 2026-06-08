package seed

import (
	"fmt"
	"log"
	"sanitation-backend/database"
	"sanitation-backend/models"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func SeedData() {
	seedAreas()
	seedUsers()
	seedRoadSections()
	seedWorkers()
	seedVehicles()
	log.Println("Seed data initialization completed")
}

func hashPassword(password string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashed)
}

func seedAreas() {
	var count int64
	database.DB.Model(&models.Area{}).Count(&count)
	if count > 0 {
		log.Println("Areas already seeded, skipping...")
		return
	}

	areas := []models.Area{
		{Name: "第一片区"},
		{Name: "第二片区"},
		{Name: "第三片区"},
	}

	for _, area := range areas {
		database.DB.Create(&area)
	}
	log.Println("Areas seeded successfully")
}

func seedUsers() {
	var count int64
	database.DB.Model(&models.User{}).Count(&count)
	if count > 0 {
		log.Println("Users already seeded, skipping...")
		return
	}

	var areas []models.Area
	database.DB.Find(&areas)

	users := []models.User{
		{
			Username: "admin",
			Password: hashPassword("sanit@2024"),
			RealName: "系统管理员",
			Role:     "admin",
		},
	}

	if len(areas) >= 3 {
		users = append(users,
			models.User{
				Username: "area1",
				Password: hashPassword("ar123"),
				RealName: "一片区长",
				Role:     "area_manager",
				AreaID:   &areas[0].ID,
			},
			models.User{
				Username: "area2",
				Password: hashPassword("ar123"),
				RealName: "二片区长",
				Role:     "area_manager",
				AreaID:   &areas[1].ID,
			},
			models.User{
				Username: "area3",
				Password: hashPassword("ar123"),
				RealName: "三片区长",
				Role:     "area_manager",
				AreaID:   &areas[2].ID,
			},
		)
	}

	for _, user := range users {
		database.DB.Create(&user)
	}
	log.Println("Users seeded successfully")
}

func seedRoadSections() {
	var count int64
	database.DB.Model(&models.RoadSection{}).Count(&count)
	if count > 0 {
		log.Println("Road sections already seeded, skipping...")
		return
	}

	var areas []models.Area
	database.DB.Find(&areas)

	if len(areas) < 3 {
		log.Println("Not enough areas to seed road sections")
		return
	}

	roads := []models.RoadSection{
		{Name: "人民路东段", AreaID: areas[0].ID, Level: 1, Length: 2.5},
		{Name: "人民路西段", AreaID: areas[0].ID, Level: 2, Length: 2.3},
		{Name: "解放大道北段", AreaID: areas[1].ID, Level: 1, Length: 3.0},
		{Name: "解放大道南段", AreaID: areas[1].ID, Level: 2, Length: 2.8},
		{Name: "中山路", AreaID: areas[2].ID, Level: 2, Length: 2.0},
		{Name: "建设街", AreaID: areas[2].ID, Level: 3, Length: 1.5},
	}

	for _, road := range roads {
		database.DB.Create(&road)
	}
	log.Println("Road sections seeded successfully")
}

func seedWorkers() {
	var count int64
	database.DB.Model(&models.Worker{}).Count(&count)
	if count > 0 {
		log.Println("Workers already seeded, skipping...")
		return
	}

	var areas []models.Area
	database.DB.Find(&areas)

	var roads []models.RoadSection
	database.DB.Find(&roads)

	if len(areas) < 3 || len(roads) < 6 {
		log.Println("Not enough areas or roads to seed workers")
		return
	}

	workerNames := []string{
		"张建国", "李秀英", "王德福", "赵桂兰",
		"刘志强", "陈美玲", "杨洪武", "黄淑芬",
		"周明辉", "吴秀珍", "郑海涛", "孙丽娟",
	}
	phones := []string{
		"13800000001", "13800000002", "13800000003", "13800000004",
		"13800000005", "13800000006", "13800000007", "13800000008",
		"13800000009", "13800000010", "13800000011", "13800000012",
	}

	areaAssignments := []uint{
		areas[0].ID, areas[0].ID, areas[0].ID, areas[0].ID,
		areas[1].ID, areas[1].ID, areas[1].ID, areas[1].ID,
		areas[2].ID, areas[2].ID, areas[2].ID, areas[2].ID,
	}

	roadAssignments := []*uint{
		&roads[0].ID, &roads[0].ID, &roads[1].ID, &roads[1].ID,
		&roads[2].ID, &roads[2].ID, &roads[3].ID, &roads[3].ID,
		&roads[4].ID, &roads[4].ID, &roads[5].ID, &roads[5].ID,
	}

	for i := 0; i < 12; i++ {
		worker := models.Worker{
			Name:          workerNames[i],
			Phone:         phones[i],
			IDCard:        fmt.Sprintf("32010119%02d0101%04d", 60+i, i+1),
			AreaID:        areaAssignments[i],
			RoadSectionID: roadAssignments[i],
			Status:        "active",
		}
		database.DB.Create(&worker)
	}
	log.Println("Workers seeded successfully")
}

func seedVehicles() {
	var count int64
	database.DB.Model(&models.Vehicle{}).Count(&count)
	if count > 0 {
		log.Println("Vehicles already seeded, skipping...")
		return
	}

	now := time.Now()
	vehicles := []models.Vehicle{
		{
			PlateNumber:           "苏A·12345",
			Type:                  "sprinkler",
			Status:                "available",
			Mileage:               12500,
			LastMaintenance:       &now,
			LastMaintenanceMileage: 10000,
		},
		{
			PlateNumber:           "苏A·12346",
			Type:                  "sprinkler",
			Status:                "available",
			Mileage:               8500,
			LastMaintenance:       &now,
			LastMaintenanceMileage: 5000,
		},
		{
			PlateNumber:           "苏A·23456",
			Type:                  "sweeper",
			Status:                "on_duty",
			Mileage:               15000,
			LastMaintenance:       &now,
			LastMaintenanceMileage: 10000,
		},
		{
			PlateNumber:           "苏A·23457",
			Type:                  "sweeper",
			Status:                "available",
			Mileage:               6000,
			LastMaintenance:       &now,
			LastMaintenanceMileage: 5000,
		},
		{
			PlateNumber:           "苏A·34567",
			Type:                  "garbage_truck",
			Status:                "maintenance",
			Mileage:               20000,
			LastMaintenance:       &now,
			LastMaintenanceMileage: 15000,
		},
	}

	for _, v := range vehicles {
		database.DB.Create(&v)
	}
	log.Println("Vehicles seeded successfully")
}
