package services

import (
	"encoding/json"
	"fmt"
	"log"

	"mqtt-bridge/database"
	"mqtt-bridge/models"

	"gorm.io/gorm"
)

type NodeService struct {
	db *database.Database
}

func NewNodeService(db *database.Database) *NodeService {
	return &NodeService{
		db: db,
	}
}

// Node Management

func (ns *NodeService) CreateNode(templateID uint, req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	// Check if template exists
	var template models.OrderTemplate
	if err := ns.db.DB.First(&template, templateID).Error; err != nil {
		return nil, fmt.Errorf("order template not found: %w", err)
	}

	// Check if node ID already exists in this template
	var existingNode models.NodeTemplate
	err := ns.db.DB.Where("order_template_id = ? AND node_id = ?", templateID, req.NodeID).First(&existingNode).Error
	if err == nil {
		return nil, fmt.Errorf("node with ID '%s' already exists in template %d", req.NodeID, templateID)
	}

	node := &models.NodeTemplate{
		OrderTemplateID:       templateID,
		NodeID:                req.NodeID,
		Description:           req.Description,
		SequenceID:            req.SequenceID,
		Released:              req.Released,
		X:                     req.Position.X,
		Y:                     req.Position.Y,
		Theta:                 req.Position.Theta,
		AllowedDeviationXY:    req.Position.AllowedDeviationXY,
		AllowedDeviationTheta: req.Position.AllowedDeviationTheta,
		MapID:                 req.Position.MapID,
	}

	// Start transaction
	tx := ns.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create node
	if err := tx.Create(node).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Create actions for node
	for _, actionReq := range req.Actions {
		if err := ns.createActionTemplate(tx, &actionReq, &node.ID, nil); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create node action: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Node Service] Node created successfully: %s (ID: %d) in template %d", node.NodeID, node.ID, templateID)

	// Fetch the complete node with actions
	return ns.GetNode(node.ID)
}

func (ns *NodeService) GetNode(nodeID uint) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := ns.db.DB.Where("id = ?", nodeID).
		Preload("Actions.Parameters").
		First(&node).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	return &node, nil
}

func (ns *NodeService) GetNodeByNodeID(templateID uint, nodeID string) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := ns.db.DB.Where("order_template_id = ? AND node_id = ?", templateID, nodeID).
		Preload("Actions.Parameters").
		First(&node).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	return &node, nil
}

func (ns *NodeService) ListNodes(templateID uint) ([]models.NodeTemplate, error) {
	var nodes []models.NodeTemplate
	err := ns.db.DB.Where("order_template_id = ?", templateID).
		Preload("Actions.Parameters").
		Order("sequence_id ASC").
		Find(&nodes).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	log.Printf("[Node Service] Retrieved %d nodes for template %d", len(nodes), templateID)
	return nodes, nil
}

func (ns *NodeService) UpdateNode(nodeID uint, req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	// Get existing node
	existingNode, err := ns.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	// Check for nodeID conflicts (if nodeID is being changed)
	if existingNode.NodeID != req.NodeID {
		var conflictNode models.NodeTemplate
		err := ns.db.DB.Where("order_template_id = ? AND node_id = ? AND id != ?",
			existingNode.OrderTemplateID, req.NodeID, nodeID).First(&conflictNode).Error
		if err == nil {
			return nil, fmt.Errorf("node with ID '%s' already exists in template", req.NodeID)
		}
	}

	// Start transaction
	tx := ns.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update node
	updateNode := &models.NodeTemplate{
		NodeID:                req.NodeID,
		Description:           req.Description,
		SequenceID:            req.SequenceID,
		Released:              req.Released,
		X:                     req.Position.X,
		Y:                     req.Position.Y,
		Theta:                 req.Position.Theta,
		AllowedDeviationXY:    req.Position.AllowedDeviationXY,
		AllowedDeviationTheta: req.Position.AllowedDeviationTheta,
		MapID:                 req.Position.MapID,
	}

	if err := tx.Model(&models.NodeTemplate{}).Where("id = ?", nodeID).Updates(updateNode).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update node: %w", err)
	}

	// Delete existing actions and parameters
	if err := ns.deleteNodeActions(tx, nodeID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete existing actions: %w", err)
	}

	// Create new actions
	for _, actionReq := range req.Actions {
		if err := ns.createActionTemplate(tx, &actionReq, &nodeID, nil); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Node Service] Node updated successfully: %s (ID: %d)", req.NodeID, nodeID)
	return ns.GetNode(nodeID)
}

func (ns *NodeService) DeleteNode(nodeID uint) error {
	// Get node info for logging
	node, err := ns.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Start transaction
	tx := ns.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete actions and parameters
	if err := ns.deleteNodeActions(tx, nodeID); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete node actions: %w", err)
	}

	// Delete node
	if err := tx.Delete(&models.NodeTemplate{}, nodeID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete node: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Node Service] Node deleted successfully: %s (ID: %d)", node.NodeID, nodeID)
	return nil
}

// Helper functions

func (ns *NodeService) createActionTemplate(tx *gorm.DB, actionReq *models.ActionTemplateRequest,
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
		valueStr, err := ns.convertValueToString(paramReq.Value, paramReq.ValueType)
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

func (ns *NodeService) convertValueToString(value interface{}, valueType string) (string, error) {
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

func (ns *NodeService) deleteNodeActions(tx *gorm.DB, nodeID uint) error {
	// Delete action parameters
	if err := tx.Exec(`
		DELETE FROM action_parameter_templates 
		WHERE action_template_id IN (
			SELECT id FROM action_templates WHERE node_template_id = ?
		)
	`, nodeID).Error; err != nil {
		return err
	}

	// Delete actions
	if err := tx.Where("node_template_id = ?", nodeID).Delete(&models.ActionTemplate{}).Error; err != nil {
		return err
	}

	return nil
}
