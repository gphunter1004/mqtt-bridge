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

// Database holds the DB connection, all repository instances, and the UnitOfWork.
type Database struct {
	DB                 *gorm.DB
	UoW                UnitOfWorkInterface
	ConnectionRepo     interfaces.ConnectionRepositoryInterface
	FactsheetRepo      interfaces.FactsheetRepositoryInterface
	ActionRepo         interfaces.ActionRepositoryInterface
	NodeRepo           interfaces.NodeRepositoryInterface
	EdgeRepo           interfaces.EdgeRepositoryInterface
	OrderTemplateRepo  interfaces.OrderTemplateRepositoryInterface
	OrderExecutionRepo interfaces.OrderExecutionRepositoryInterface
}

// NewDatabase creates a new database connection and initializes repositories.
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
		&models.ConnectionState{}, &models.ConnectionStateHistory{},
		&models.AgvAction{}, &models.AgvActionParameter{},
		&models.PhysicalParameter{}, &models.TypeSpecification{},
		&models.ActionTemplate{}, &models.ActionParameterTemplate{},
		&models.OrderTemplate{}, &models.OrderExecution{},
		&models.NodeTemplate{}, &models.EdgeTemplate{},
		&models.OrderTemplateNode{}, &models.OrderTemplateEdge{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}
	log.Println("[DB] Database migration completed successfully")

	// Initialize repositories
	connectionRepo := repositories.NewConnectionRepository(db)
	factsheetRepo := repositories.NewFactsheetRepository(db)
	actionRepo := repositories.NewActionRepository(db)
	nodeRepo := repositories.NewNodeRepository(db)
	edgeRepo := repositories.NewEdgeRepository(db)
	orderTemplateRepo := repositories.NewOrderTemplateRepository(db)
	orderExecutionRepo := repositories.NewOrderExecutionRepository(db)

	return &Database{
		DB:                 db,
		UoW:                NewUnitOfWork(db), // Initialize UnitOfWork here
		ConnectionRepo:     connectionRepo,
		FactsheetRepo:      factsheetRepo,
		ActionRepo:         actionRepo,
		NodeRepo:           nodeRepo,
		EdgeRepo:           edgeRepo,
		OrderTemplateRepo:  orderTemplateRepo,
		OrderExecutionRepo: orderExecutionRepo,
	}, nil
}

// Legacy methods for backward compatibility, though direct repository usage is preferred.
func (d *Database) SaveConnectionState(tx *gorm.DB, connectionMsg *models.ConnectionMessage) error {
	return d.ConnectionRepo.SaveConnectionState(tx, connectionMsg)
}

func (d *Database) SaveOrUpdateFactsheet(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	return d.FactsheetRepo.SaveOrUpdateFactsheet(tx, factsheetMsg)
}

func (d *Database) GetLastConnectionState(serialNumber string) (*models.ConnectionState, error) {
	return d.ConnectionRepo.GetLastConnectionState(serialNumber)
}

func (d *Database) DebugAgvActions(serialNumber string) {
	d.FactsheetRepo.DebugAgvActions(serialNumber)
}
