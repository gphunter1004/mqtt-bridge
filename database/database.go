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
		// Existing models
		&models.ConnectionState{},
		&models.ConnectionStateHistory{},
		&models.AgvAction{},
		&models.AgvActionParameter{},
		&models.PhysicalParameter{},
		&models.TypeSpecification{},
		// New order management models
		&models.OrderTemplate{},
		&models.NodeTemplate{},
		&models.EdgeTemplate{},
		&models.ActionTemplate{},
		&models.ActionParameterTemplate{},
		&models.OrderExecution{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("[DB] Database migration completed successfully")

	// Check if tables exist and log their structure
	var tableNames []string
	db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableNames)
	log.Printf("[DB] Created tables: %v", tableNames)

	return &Database{DB: db}, nil
}

func (d *Database) SaveConnectionState(connectionMsg *models.ConnectionMessage) error {
	// Start transaction
	tx := d.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	connectionState := &models.ConnectionState{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}

	// Save/Update current connection state
	err := tx.Where("serial_number = ?", connectionMsg.SerialNumber).
		Assign(connectionState).
		FirstOrCreate(connectionState).Error

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to save connection state: %w", err)
	}

	// Always save to history table
	connectionHistory := &models.ConnectionStateHistory{
		SerialNumber:    connectionMsg.SerialNumber,
		ConnectionState: connectionMsg.ConnectionState,
		HeaderID:        connectionMsg.HeaderID,
		Timestamp:       connectionMsg.Timestamp,
		Version:         connectionMsg.Version,
		Manufacturer:    connectionMsg.Manufacturer,
	}

	err = tx.Create(connectionHistory).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to save connection history: %w", err)
	}

	// Commit transaction
	err = tx.Commit().Error
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[DB] Connection state saved/updated for robot: %s, state: %s",
		connectionMsg.SerialNumber, connectionMsg.ConnectionState)
	return nil
}

func (d *Database) SaveOrUpdateFactsheet(factsheetMsg *models.FactsheetMessage) error {
	log.Printf("🎯🎯🎯 NEW FACTSHEET SAVE FUNCTION CALLED 🎯🎯🎯")

	// Start transaction
	tx := d.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Save Physical Parameters
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

	err := tx.Where("serial_number = ?", factsheetMsg.SerialNumber).
		Assign(physicalParam).
		FirstOrCreate(physicalParam).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to save physical parameters: %w", err)
	}
	log.Printf("[DB] Physical parameters saved for robot: %s", factsheetMsg.SerialNumber)

	// Save Type Specification
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

	err = tx.Where("serial_number = ?", factsheetMsg.SerialNumber).
		Assign(typeSpec).
		FirstOrCreate(typeSpec).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to save type specification: %w", err)
	}
	log.Printf("[DB] Type specification saved for robot: %s", factsheetMsg.SerialNumber)

	// Save AGV Actions and Parameters
	log.Printf("🎯🎯🎯 PROCESSING %d AGV ACTIONS 🎯🎯🎯", len(factsheetMsg.ProtocolFeatures.AgvActions))

	for i, action := range factsheetMsg.ProtocolFeatures.AgvActions {
		log.Printf("🎯🎯🎯 ACTION %d: %s (%d params) 🎯🎯🎯", i+1, action.ActionType, len(action.ActionParameters))

		actionScopesJSON, _ := json.Marshal(action.ActionScopes)

		agvAction := &models.AgvAction{
			SerialNumber:      factsheetMsg.SerialNumber,
			ActionType:        action.ActionType,
			ActionDescription: action.ActionDescription,
			ActionScopes:      string(actionScopesJSON),
			ResultDescription: action.ResultDescription,
		}

		// Check if action already exists
		var existingAction models.AgvAction
		err := tx.Where("serial_number = ? AND action_type = ?",
			factsheetMsg.SerialNumber, action.ActionType).First(&existingAction).Error

		if err == gorm.ErrRecordNotFound {
			// Create new action
			err = tx.Create(agvAction).Error
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create agv action %s: %w", action.ActionType, err)
			}
			log.Printf("🎯🎯🎯 NEW ACTION CREATED: %s (ID: %d) 🎯🎯🎯", action.ActionType, agvAction.ID)

			// Save parameters for new action
			for _, param := range action.ActionParameters {
				log.Printf("🎯🎯🎯 CREATING PARAM: %s 🎯🎯🎯", param.Key)

				actionParam := &models.AgvActionParameter{
					AgvActionID:   agvAction.ID,
					Key:           param.Key,
					Description:   param.Description,
					IsOptional:    param.IsOptional,
					ValueDataType: param.ValueDataType,
				}

				err = tx.Create(actionParam).Error
				if err != nil {
					log.Printf("🎯🎯🎯 ERROR CREATING PARAM: %v 🎯🎯🎯", err)
					tx.Rollback()
					return fmt.Errorf("failed to create parameter %s: %w", param.Key, err)
				}
				log.Printf("🎯🎯🎯 PARAM CREATED: %s (ID: %d) 🎯🎯🎯", param.Key, actionParam.ID)
			}

		} else if err == nil {
			// Update existing action
			agvAction.ID = existingAction.ID
			err = tx.Save(agvAction).Error
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update agv action %s: %w", action.ActionType, err)
			}
			log.Printf("🎯🎯🎯 ACTION UPDATED: %s (ID: %d) 🎯🎯🎯", action.ActionType, agvAction.ID)

			// Smart parameter update
			var existingParams []models.AgvActionParameter
			err = tx.Where("agv_action_id = ?", agvAction.ID).Find(&existingParams).Error
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to get existing parameters: %w", err)
			}

			log.Printf("🎯🎯🎯 FOUND %d EXISTING PARAMS 🎯🎯🎯", len(existingParams))

			// Create map of existing parameters
			existingParamMap := make(map[string]models.AgvActionParameter)
			for _, param := range existingParams {
				existingParamMap[param.Key] = param
			}

			// Track which parameters to keep
			keepParamKeys := make(map[string]bool)

			// Process each parameter from factsheet
			for _, param := range action.ActionParameters {
				keepParamKeys[param.Key] = true

				if existingParam, exists := existingParamMap[param.Key]; exists {
					// Parameter exists, check if update needed
					needsUpdate := existingParam.Description != param.Description ||
						existingParam.IsOptional != param.IsOptional ||
						existingParam.ValueDataType != param.ValueDataType

					if needsUpdate {
						existingParam.Description = param.Description
						existingParam.IsOptional = param.IsOptional
						existingParam.ValueDataType = param.ValueDataType

						err = tx.Save(&existingParam).Error
						if err != nil {
							tx.Rollback()
							return fmt.Errorf("failed to update parameter %s: %w", param.Key, err)
						}
						log.Printf("🎯🎯🎯 PARAM UPDATED: %s (ID: %d) 🎯🎯🎯", param.Key, existingParam.ID)
					} else {
						log.Printf("🎯🎯🎯 PARAM UNCHANGED: %s (ID: %d) 🎯🎯🎯", param.Key, existingParam.ID)
					}
				} else {
					// New parameter
					newParam := &models.AgvActionParameter{
						AgvActionID:   agvAction.ID,
						Key:           param.Key,
						Description:   param.Description,
						IsOptional:    param.IsOptional,
						ValueDataType: param.ValueDataType,
					}

					err = tx.Create(newParam).Error
					if err != nil {
						tx.Rollback()
						return fmt.Errorf("failed to create new parameter %s: %w", param.Key, err)
					}
					log.Printf("🎯🎯🎯 NEW PARAM CREATED: %s (ID: %d) 🎯🎯🎯", param.Key, newParam.ID)
				}
			}

			// Delete obsolete parameters
			for _, existingParam := range existingParams {
				if !keepParamKeys[existingParam.Key] {
					err = tx.Delete(&existingParam).Error
					if err != nil {
						tx.Rollback()
						return fmt.Errorf("failed to delete obsolete parameter %s: %w", existingParam.Key, err)
					}
					log.Printf("🎯🎯🎯 OBSOLETE PARAM DELETED: %s (ID: %d) 🎯🎯🎯", existingParam.Key, existingParam.ID)
				}
			}

		} else {
			tx.Rollback()
			return fmt.Errorf("failed to query agv action %s: %w", action.ActionType, err)
		}
	}

	// Commit transaction
	err = tx.Commit().Error
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("🎯🎯🎯 TRANSACTION COMMITTED SUCCESSFULLY 🎯🎯🎯")

	// Verify parameters were saved
	var totalParams int64
	d.DB.Model(&models.AgvActionParameter{}).
		Joins("JOIN agv_actions ON agv_actions.id = agv_action_parameters.agv_action_id").
		Where("agv_actions.serial_number = ?", factsheetMsg.SerialNumber).
		Count(&totalParams)

	log.Printf("🎯🎯🎯 FINAL PARAM COUNT: %d 🎯🎯🎯", totalParams)

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

// Debug function to check AGV actions and parameters
func (d *Database) DebugAgvActions(serialNumber string) {
	log.Printf("[DB DEBUG] Checking AGV actions for robot: %s", serialNumber)

	var actions []models.AgvAction
	err := d.DB.Where("serial_number = ?", serialNumber).
		Preload("Parameters").Find(&actions).Error

	if err != nil {
		log.Printf("[DB DEBUG ERROR] Failed to query AGV actions: %v", err)
		return
	}

	log.Printf("[DB DEBUG] Found %d AGV actions for robot: %s", len(actions), serialNumber)

	for i, action := range actions {
		log.Printf("[DB DEBUG] Action %d:", i+1)
		log.Printf("[DB DEBUG]   ID: %d", action.ID)
		log.Printf("[DB DEBUG]   Type: %s", action.ActionType)
		log.Printf("[DB DEBUG]   Description: %s", action.ActionDescription)
		log.Printf("[DB DEBUG]   Parameters count: %d", len(action.Parameters))

		for j, param := range action.Parameters {
			log.Printf("[DB DEBUG]     Parameter %d:", j+1)
			log.Printf("[DB DEBUG]       ID: %d", param.ID)
			log.Printf("[DB DEBUG]       Key: %s", param.Key)
			log.Printf("[DB DEBUG]       Description: %s", param.Description)
			log.Printf("[DB DEBUG]       DataType: %s", param.ValueDataType)
			log.Printf("[DB DEBUG]       Optional: %t", param.IsOptional)
		}
	}

	// Also check parameters table directly
	var allParams []models.AgvActionParameter
	err = d.DB.Joins("JOIN agv_actions ON agv_actions.id = agv_action_parameters.agv_action_id").
		Where("agv_actions.serial_number = ?", serialNumber).
		Find(&allParams).Error

	if err != nil {
		log.Printf("[DB DEBUG ERROR] Failed to query parameters directly: %v", err)
		return
	}

	log.Printf("[DB DEBUG] Total parameters in database for robot %s: %d", serialNumber, len(allParams))
}
