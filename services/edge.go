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
	db            *database.Database
	actionService *ActionService
}

func NewEdgeService(db *database.Database) *EdgeService {
	return &EdgeService{
		db:            db,
		actionService: NewActionService(db),
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

	// Create independent action templates and collect their IDs
	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := es.createActionTemplateInTx(tx, &actionReq)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action template: %w", err)
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	// Set action template IDs in edge
	if err := edge.SetActionTemplateIDs(actionTemplateIDs); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to set action template IDs: %w", err)
	}

	// Create edge
	if err := tx.Create(edge).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create edge: %w", err)
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
	err := es.db.DB.Where("id = ?", edgeID).First(&edge).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	return &edge, nil
}

func (es *EdgeService) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	var edge models.EdgeTemplate
	err := es.db.DB.Where("edge_id = ?", edgeID).First(&edge).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	return &edge, nil
}

func (es *EdgeService) GetEdgeWithActions(edgeID uint) (*EdgeWithActions, error) {
	edge, err := es.GetEdge(edgeID)
	if err != nil {
		return nil, err
	}

	// Get associated action templates
	actionIDs, err := edge.GetActionTemplateIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse action template IDs: %w", err)
	}

	var actions []models.ActionTemplate
	if len(actionIDs) > 0 {
		err = es.db.DB.Where("id IN ?", actionIDs).
			Preload("Parameters").
			Find(&actions).Error
		if err != nil {
			return nil, fmt.Errorf("failed to get actions: %w", err)
		}
	}

	return &EdgeWithActions{
		EdgeTemplate: *edge,
		Actions:      actions,
	}, nil
}

func (es *EdgeService) ListEdges(limit, offset int) ([]models.EdgeTemplate, error) {
	var edges []models.EdgeTemplate
	query := es.db.DB.Order("created_at DESC")

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

	// Delete old action templates if they exist
	oldActionIDs, err := existingEdge.GetActionTemplateIDs()
	if err == nil && len(oldActionIDs) > 0 {
		if err := es.deleteActionTemplatesInTx(tx, oldActionIDs); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete old action templates: %w", err)
		}
	}

	// Create new action templates
	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := es.createActionTemplateInTx(tx, &actionReq)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action template: %w", err)
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

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

	// Set new action template IDs
	if err := updateEdge.SetActionTemplateIDs(actionTemplateIDs); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to set action template IDs: %w", err)
	}

	if err := tx.Model(&models.EdgeTemplate{}).Where("id = ?", edgeID).Updates(updateEdge).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update edge: %w", err)
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

	// Delete associated action templates
	actionIDs, err := edge.GetActionTemplateIDs()
	if err == nil && len(actionIDs) > 0 {
		if err := es.deleteActionTemplatesInTx(tx, actionIDs); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete action templates: %w", err)
		}
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

func (es *EdgeService) createActionTemplateInTx(tx *gorm.DB, actionReq *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	action := &models.ActionTemplate{
		ActionType:        actionReq.ActionType,
		ActionID:          actionReq.ActionID,
		BlockingType:      actionReq.BlockingType,
		ActionDescription: actionReq.ActionDescription,
	}

	if err := tx.Create(action).Error; err != nil {
		return nil, err
	}

	// Create action parameters
	for _, paramReq := range actionReq.Parameters {
		valueStr, err := es.convertValueToString(paramReq.Value, paramReq.ValueType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameter value: %w", err)
		}

		param := &models.ActionParameterTemplate{
			ActionTemplateID: action.ID,
			Key:              paramReq.Key,
			Value:            valueStr,
			ValueType:        paramReq.ValueType,
		}

		if err := tx.Create(param).Error; err != nil {
			return nil, err
		}
	}

	return action, nil
}

func (es *EdgeService) deleteActionTemplatesInTx(tx *gorm.DB, actionIDs []uint) error {
	// Delete action parameters
	if err := tx.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{}).Error; err != nil {
		return err
	}

	// Delete actions
	if err := tx.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{}).Error; err != nil {
		return err
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

// Helper structures are defined in types.go
