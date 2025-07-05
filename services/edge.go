package services

import (
	"encoding/json"
	"fmt"
	"log"

	"mqtt-bridge/database"
	"mqtt-bridge/models"

	"gorm.io/gorm"
)

type EdgeService struct {
	db *database.Database
}

func NewEdgeService(db *database.Database) *EdgeService {
	return &EdgeService{
		db: db,
	}
}

// Edge Management

func (es *EdgeService) CreateEdge(req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	// Check if edge ID already exists
	var existingEdge models.EdgeTemplate
	err := es.db.DB.Where("edge_id = ?", req.EdgeID).First(&existingEdge).Error
	if err == nil {
		return nil, fmt.Errorf("edge with ID '%s' already exists", req.EdgeID)
	}

	edge := &models.EdgeTemplate{
		EdgeID:      req.EdgeID,
		Name:        req.Name,
		Description: req.Description,
		SequenceID:  req.SequenceID,
		Released:    req.Released,
		StartNodeID: req.StartNodeID,
		EndNodeID:   req.EndNodeID,
	}

	// Start transaction
	tx := es.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create edge
	if err := tx.Create(edge).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create edge: %w", err)
	}

	// Create actions for edge
	for _, actionReq := range req.Actions {
		if err := es.createActionTemplate(tx, &actionReq, nil, &edge.ID); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create edge action: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Edge Service] Edge created successfully: %s (ID: %d)", edge.EdgeID, edge.ID)

	// Fetch the complete edge with actions
	return es.GetEdge(edge.ID)
}

func (es *EdgeService) GetEdge(edgeID uint) (*models.EdgeTemplate, error) {
	var edge models.EdgeTemplate
	err := es.db.DB.Where("id = ?", edgeID).
		Preload("Actions.Parameters").
		First(&edge).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	return &edge, nil
}

func (es *EdgeService) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	var edge models.EdgeTemplate
	err := es.db.DB.Where("edge_id = ?", edgeID).
		Preload("Actions.Parameters").
		First(&edge).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	return &edge, nil
}

func (es *EdgeService) ListEdges(limit, offset int) ([]models.EdgeTemplate, error) {
	var edges []models.EdgeTemplate
	query := es.db.DB.Preload("Actions.Parameters").Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&edges).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list edges: %w", err)
	}

	log.Printf("[Edge Service] Retrieved %d edges", len(edges))
	return edges, nil
}

func (es *EdgeService) UpdateEdge(edgeID uint, req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	// Get existing edge
	existingEdge, err := es.GetEdge(edgeID)
	if err != nil {
		return nil, fmt.Errorf("edge not found: %w", err)
	}

	// Check for edgeID conflicts (if edgeID is being changed)
	if existingEdge.EdgeID != req.EdgeID {
		var conflictEdge models.EdgeTemplate
		err := es.db.DB.Where("edge_id = ? AND id != ?",
			req.EdgeID, edgeID).First(&conflictEdge).Error
		if err == nil {
			return nil, fmt.Errorf("edge with ID '%s' already exists", req.EdgeID)
		}
	}

	// Start transaction
	tx := es.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update edge
	updateEdge := &models.EdgeTemplate{
		EdgeID:      req.EdgeID,
		Name:        req.Name,
		Description: req.Description,
		SequenceID:  req.SequenceID,
		Released:    req.Released,
		StartNodeID: req.StartNodeID,
		EndNodeID:   req.EndNodeID,
	}

	if err := tx.Model(&models.EdgeTemplate{}).Where("id = ?", edgeID).Updates(updateEdge).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update edge: %w", err)
	}

	// Delete existing actions and parameters
	if err := es.deleteEdgeActions(tx, edgeID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete existing actions: %w", err)
	}

	// Create new actions
	for _, actionReq := range req.Actions {
		if err := es.createActionTemplate(tx, &actionReq, nil, &edgeID); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Edge Service] Edge updated successfully: %s (ID: %d)", req.EdgeID, edgeID)
	return es.GetEdge(edgeID)
}

func (es *EdgeService) DeleteEdge(edgeID uint) error {
	// Get edge info for logging
	edge, err := es.GetEdge(edgeID)
	if err != nil {
		return fmt.Errorf("edge not found: %w", err)
	}

	// Start transaction
	tx := es.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete actions and parameters
	if err := es.deleteEdgeActions(tx, edgeID); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete edge actions: %w", err)
	}

	// Delete template associations
	if err := tx.Where("edge_template_id = ?", edgeID).Delete(&models.OrderTemplateEdge{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete template associations: %w", err)
	}

	// Delete edge
	if err := tx.Delete(&models.EdgeTemplate{}, edgeID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete edge: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Edge Service] Edge deleted successfully: %s (ID: %d)", edge.EdgeID, edgeID)
	return nil
}

// Helper functions

func (es *EdgeService) createActionTemplate(tx *gorm.DB, actionReq *models.ActionTemplateRequest,
	nodeID *uint, edgeID *uint) error {

	action := &models.ActionTemplate{
		NodeTemplateID:    nodeID,
		EdgeTemplateID:    edgeID,
		ActionType:        actionReq.ActionType,
		ActionID:          actionReq.ActionID,
		BlockingType:      actionReq.BlockingType,
		ActionDescription: actionReq.ActionDescription,
	}

	if err := tx.Create(action).Error; err != nil {
		return err
	}

	// Create action parameters
	for _, paramReq := range actionReq.Parameters {
		// Convert value to JSON string based on type
		valueStr, err := es.convertValueToString(paramReq.Value, paramReq.ValueType)
		if err != nil {
			return fmt.Errorf("failed to convert parameter value: %w", err)
		}

		param := &models.ActionParameterTemplate{
			ActionTemplateID: action.ID,
			Key:              paramReq.Key,
			Value:            valueStr,
			ValueType:        paramReq.ValueType,
		}

		if err := tx.Create(param).Error; err != nil {
			return err
		}
	}

	return nil
}

func (es *EdgeService) convertValueToString(value interface{}, valueType string) (string, error) {
	if value == nil {
		return "", nil
	}

	switch valueType {
	case "string":
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
	case "object", "number", "boolean":
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

func (es *EdgeService) deleteEdgeActions(tx *gorm.DB, edgeID uint) error {
	// Delete action parameters
	if err := tx.Exec(`
		DELETE FROM action_parameter_templates 
		WHERE action_template_id IN (
			SELECT id FROM action_templates WHERE edge_template_id = ?
		)
	`, edgeID).Error; err != nil {
		return err
	}

	// Delete actions
	if err := tx.Where("edge_template_id = ?", edgeID).Delete(&models.ActionTemplate{}).Error; err != nil {
		return err
	}

	return nil
}
