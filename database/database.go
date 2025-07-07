package database

import (
	"fmt"
	"log"

	"mqtt-bridge/config"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	DB *gorm.DB

	// Repository interfaces
	ConnectionRepo     interfaces.ConnectionRepositoryInterface
	FactsheetRepo      interfaces.FactsheetRepositoryInterface
	ActionRepo         interfaces.ActionRepositoryInterface
	NodeRepo           interfaces.NodeRepositoryInterface
	EdgeRepo           interfaces.EdgeRepositoryInterface
	OrderTemplateRepo  interfaces.OrderTemplateRepositoryInterface
	OrderExecutionRepo interfaces.OrderExecutionRepositoryInterface
}

func NewDatabase(cfg *config.Config) (*Database, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Seoul",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

	log.Printf("[DB] Connecting to database: %s@%s:%s/%s", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("[DB] Database connected successfully")

	// Auto migrate the schema
	log.Println("[DB] Starting database migration...")
	err = db.AutoMigrate(
		&models.ConnectionState{},
		&models.ConnectionStateHistory{},
		&models.AgvAction{},
		&models.AgvActionParameter{},
		&models.PhysicalParameter{},
		&models.TypeSpecification{},
		&models.ActionTemplate{},
		&models.ActionParameterTemplate{},
		&models.OrderTemplate{},
		&models.OrderExecution{},
		&models.NodeTemplate{},
		&models.EdgeTemplate{},
		&models.OrderTemplateNode{},
		&models.OrderTemplateEdge{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("[DB] Database migration completed successfully")

	var tableNames []string
	db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableNames)
	log.Printf("[DB] Created tables: %v", tableNames)

	// Initialize repositories
	connectionRepo := repositories.NewConnectionRepository(db)
	factsheetRepo := repositories.NewFactsheetRepository(db)
	actionRepo := repositories.NewActionRepository(db)
	nodeRepo := repositories.NewNodeRepository(db)
	edgeRepo := repositories.NewEdgeRepository(db)
	orderTemplateRepo := repositories.NewOrderTemplateRepository(db)
	orderExecutionRepo := repositories.NewOrderExecutionRepository(db)

	return &Database{
		DB: db,

		// Assign repository interfaces
		ConnectionRepo:     connectionRepo,
		FactsheetRepo:      factsheetRepo,
		ActionRepo:         actionRepo,
		NodeRepo:           nodeRepo,
		EdgeRepo:           edgeRepo,
		OrderTemplateRepo:  orderTemplateRepo,
		OrderExecutionRepo: orderExecutionRepo,
	}, nil
}

// Legacy methods for backward compatibility (deprecated - use repositories instead)
// These methods delegate to the appropriate repositories

func (d *Database) SaveConnectionState(connectionMsg *models.ConnectionMessage) error {
	return d.ConnectionRepo.SaveConnectionState(connectionMsg)
}

func (d *Database) SaveOrUpdateFactsheet(factsheetMsg *models.FactsheetMessage) error {
	return d.FactsheetRepo.SaveOrUpdateFactsheet(factsheetMsg)
}

func (d *Database) GetLastConnectionState(serialNumber string) (*models.ConnectionState, error) {
	return d.ConnectionRepo.GetLastConnectionState(serialNumber)
}

func (d *Database) DebugAgvActions(serialNumber string) {
	d.FactsheetRepo.DebugAgvActions(serialNumber)
}
