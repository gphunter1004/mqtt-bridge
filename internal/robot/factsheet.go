// internal/robot/factsheet.go
package robot

import (
	"encoding/json"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"time"

	"gorm.io/gorm"
)

// FactsheetManager 팩트시트 관리
type FactsheetManager struct {
	db *gorm.DB
}

// NewFactsheetManager 새 팩트시트 관리자 생성
func NewFactsheetManager(db *gorm.DB) *FactsheetManager {
	return &FactsheetManager{
		db: db,
	}
}

// SaveFactsheet 팩트시트 저장
func (f *FactsheetManager) SaveFactsheet(resp *models.FactsheetResponse) error {
	if resp == nil {
		utils.Logger.Errorf("Factsheet response is nil")
		return nil
	}

	timestamp, _ := time.Parse(time.RFC3339, resp.Timestamp)
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// 안전한 필드 접근을 위한 헬퍼 함수들
	getStringField := func(field string) string {
		if field == "" {
			return "unknown"
		}
		return field
	}

	getIntField := func(field int) int {
		if field < 0 {
			return 0
		}
		return field
	}

	getFloatField := func(field float64) float64 {
		if field < 0 {
			return 0.0
		}
		return field
	}

	// TypeSpecification 안전 접근
	var seriesName, seriesDescription, agvClass, agvKinematics string
	var maxLoadMass int
	var localizationTypesJSON, navigationTypesJSON []byte

	if resp.TypeSpecification.SeriesName != "" {
		seriesName = resp.TypeSpecification.SeriesName
	} else {
		seriesName = "Unknown"
	}

	if resp.TypeSpecification.SeriesDescription != "" {
		seriesDescription = resp.TypeSpecification.SeriesDescription
	} else {
		seriesDescription = "No description available"
	}

	agvClass = getStringField(resp.TypeSpecification.AgvClass)
	agvKinematics = getStringField(resp.TypeSpecification.AgvKinematics)
	maxLoadMass = getIntField(resp.TypeSpecification.MaxLoadMass)

	// 배열 필드 안전 처리
	if len(resp.TypeSpecification.LocalizationTypes) > 0 {
		localizationTypesJSON, _ = json.Marshal(resp.TypeSpecification.LocalizationTypes)
	} else {
		localizationTypesJSON = []byte("[]")
	}

	if len(resp.TypeSpecification.NavigationTypes) > 0 {
		navigationTypesJSON, _ = json.Marshal(resp.TypeSpecification.NavigationTypes)
	} else {
		navigationTypesJSON = []byte("[]")
	}

	// PhysicalParameters 안전 접근
	speedMax := getFloatField(resp.PhysicalParameters.SpeedMax)
	speedMin := getFloatField(resp.PhysicalParameters.SpeedMin)
	accelerationMax := getFloatField(resp.PhysicalParameters.AccelerationMax)
	decelerationMax := getFloatField(resp.PhysicalParameters.DecelerationMax)
	length := getFloatField(resp.PhysicalParameters.Length)
	width := getFloatField(resp.PhysicalParameters.Width)
	heightMax := getFloatField(resp.PhysicalParameters.HeightMax)
	heightMin := getFloatField(resp.PhysicalParameters.HeightMin)

	factsheet := &models.RobotFactsheet{
		SerialNumber:      getStringField(resp.SerialNumber),
		Manufacturer:      getStringField(resp.Manufacturer),
		Version:           getStringField(resp.Version),
		SeriesName:        seriesName,
		SeriesDescription: seriesDescription,
		AgvClass:          agvClass,
		AgvKinematics:     agvKinematics,
		MaxLoadMass:       maxLoadMass,
		SpeedMax:          speedMax,
		SpeedMin:          speedMin,
		AccelerationMax:   accelerationMax,
		DecelerationMax:   decelerationMax,
		Length:            length,
		Width:             width,
		HeightMax:         heightMax,
		HeightMin:         heightMin,
		LocalizationTypes: string(localizationTypesJSON),
		NavigationTypes:   string(navigationTypesJSON),
		LastUpdated:       timestamp,
	}

	// 기존 팩트시트 확인
	var existing models.RobotFactsheet
	result := f.db.Where("serial_number = ?", resp.SerialNumber).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// 새로 생성
		if err := f.db.Create(factsheet).Error; err != nil {
			utils.Logger.Errorf("Failed to create factsheet: %v", err)
			return err
		}
		utils.Logger.Infof("Factsheet created for robot: %s (Series: %s, Class: %s, Kinematics: %s)",
			factsheet.SerialNumber, factsheet.SeriesName,
			factsheet.AgvClass, factsheet.AgvKinematics)
	} else if result.Error == nil {
		// 기존 업데이트
		if err := f.db.Model(&existing).Updates(factsheet).Error; err != nil {
			utils.Logger.Errorf("Failed to update factsheet: %v", err)
			return err
		}
		utils.Logger.Infof("Factsheet updated for robot: %s (Series: %s, Class: %s, Kinematics: %s)",
			factsheet.SerialNumber, factsheet.SeriesName,
			factsheet.AgvClass, factsheet.AgvKinematics)
	} else {
		utils.Logger.Errorf("Database error: %v", result.Error)
		return result.Error
	}

	return nil
}

// GetFactsheet 팩트시트 조회
func (f *FactsheetManager) GetFactsheet(serialNumber string) (*models.RobotFactsheet, error) {
	var factsheet models.RobotFactsheet
	err := f.db.Where("serial_number = ?", serialNumber).First(&factsheet).Error
	if err != nil {
		return nil, err
	}
	return &factsheet, nil
}

// GetAllFactsheets 모든 팩트시트 조회
func (f *FactsheetManager) GetAllFactsheets() ([]models.RobotFactsheet, error) {
	var factsheets []models.RobotFactsheet
	err := f.db.Find(&factsheets).Error
	return factsheets, err
}

// DeleteFactsheet 팩트시트 삭제
func (f *FactsheetManager) DeleteFactsheet(serialNumber string) error {
	return f.db.Where("serial_number = ?", serialNumber).Delete(&models.RobotFactsheet{}).Error
}

// GetFactsheetsByManufacturer 제조사별 팩트시트 조회
func (f *FactsheetManager) GetFactsheetsByManufacturer(manufacturer string) ([]models.RobotFactsheet, error) {
	var factsheets []models.RobotFactsheet
	err := f.db.Where("manufacturer = ?", manufacturer).Find(&factsheets).Error
	return factsheets, err
}

// IsFactsheetExists 팩트시트 존재 여부 확인
func (f *FactsheetManager) IsFactsheetExists(serialNumber string) bool {
	var count int64
	f.db.Model(&models.RobotFactsheet{}).Where("serial_number = ?", serialNumber).Count(&count)
	return count > 0
}
