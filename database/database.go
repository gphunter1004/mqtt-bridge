package database

import (
	"encoding/json"
	"fmt"
	"log"

	"mqtt-bridge/config"
	"mqtt-bridge/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	DB *gorm.DB
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

	return &Database{DB: db}, nil
}

func (d *Database) SaveConnectionState(connectionMsg *models.ConnectionMessage) error {
	connectionState := &models.ConnectionState{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}

	err := d.DB.Where("serial_number = ?", connectionMsg.SerialNumber).
		Assign(connectionState).
		FirstOrCreate(connectionState).Error

	if err != nil {
		return fmt.Errorf("failed to save connection state: %w", err)
	}

	connectionHistory := &models.ConnectionStateHistory{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}

	d.DB.Create(connectionHistory)
	return nil
}

func (d *Database) SaveOrUpdateFactsheet(factsheetMsg *models.FactsheetMessage) error {
	// Delete existing data
	d.DB.Exec(`DELETE FROM agv_action_parameters 
		WHERE agv_action_id IN (
			SELECT id FROM agv_actions WHERE serial_number = ?
		)`, factsheetMsg.SerialNumber)
	d.DB.Where("serial_number = ?", factsheetMsg.SerialNumber).Delete(&models.AgvAction{})
	d.DB.Where("serial_number = ?", factsheetMsg.SerialNumber).Delete(&models.PhysicalParameter{})
	d.DB.Where("serial_number = ?", factsheetMsg.SerialNumber).Delete(&models.TypeSpecification{})

	// Save new data
	physicalParam := &models.PhysicalParameter{
		SerialNumber:    factsheetMsg.SerialNumber,
		AccelerationMax: factsheetMsg.PhysicalParameters.AccelerationMax,
		DecelerationMax: factsheetMsg.PhysicalParameters.DecelerationMax,
		HeightMax:       factsheetMsg.PhysicalParameters.HeightMax,
		HeightMin:       factsheetMsg.PhysicalParameters.HeightMin,
		Length:          factsheetMsg.PhysicalParameters.Length,
		SpeedMax:        factsheetMsg.PhysicalParameters.SpeedMax,
		SpeedMin:        factsheetMsg.PhysicalParameters.SpeedMin,
		Width:           factsheetMsg.PhysicalParameters.Width,
	}
	d.DB.Create(physicalParam)

	localizationTypesJSON, _ := json.Marshal(factsheetMsg.TypeSpecification.LocalizationTypes)
	navigationTypesJSON, _ := json.Marshal(factsheetMsg.TypeSpecification.NavigationTypes)

	typeSpec := &models.TypeSpecification{
		SerialNumber:      factsheetMsg.SerialNumber,
		AgvClass:          factsheetMsg.TypeSpecification.AgvClass,
		AgvKinematics:     factsheetMsg.TypeSpecification.AgvKinematics,
		LocalizationTypes: string(localizationTypesJSON),
		MaxLoadMass:       factsheetMsg.TypeSpecification.MaxLoadMass,
		NavigationTypes:   string(navigationTypesJSON),
		SeriesDescription: factsheetMsg.TypeSpecification.SeriesDescription,
		SeriesName:        factsheetMsg.TypeSpecification.SeriesName,
	}
	d.DB.Create(typeSpec)

	for _, action := range factsheetMsg.ProtocolFeatures.AgvActions {
		actionScopesJSON, _ := json.Marshal(action.ActionScopes)

		agvAction := &models.AgvAction{
			SerialNumber:      factsheetMsg.SerialNumber,
			ActionType:        action.ActionType,
			ActionDescription: action.ActionDescription,
			ActionScopes:      string(actionScopesJSON),
			ResultDescription: action.ResultDescription,
		}

		if err := d.DB.Create(agvAction).Error; err != nil {
			continue
		}

		for _, param := range action.ActionParameters {
			actionParam := &models.AgvActionParameter{
				AgvActionID:   agvAction.ID,
				Key:           param.Key,
				Description:   param.Description,
				IsOptional:    param.IsOptional,
				ValueDataType: param.ValueDataType,
			}
			d.DB.Create(actionParam)
		}
	}

	return nil
}

func (d *Database) GetLastConnectionState(serialNumber string) (*models.ConnectionState, error) {
	var connectionState models.ConnectionState
	err := d.DB.Where("serial_number = ?", serialNumber).
		Order("created_at desc").
		First(&connectionState).Error

	if err != nil {
		return nil, err
	}

	return &connectionState, nil
}

func (d *Database) DebugAgvActions(serialNumber string) {
	var actions []models.AgvAction
	d.DB.Where("serial_number = ?", serialNumber).
		Preload("Parameters").Find(&actions)

	log.Printf("[DB DEBUG] Found %d AGV actions for robot: %s", len(actions), serialNumber)
}
