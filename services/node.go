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
	db            *database.Database
	actionService *ActionService
}

func NewNodeService(db *database.Database) *NodeService {
	return &NodeService{
		db:            db,
		actionService: NewActionService(db),
	}
}

// Node Management

func (ns *NodeService) CreateNode(req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	// Check if node ID already exists
	var existingNode models.NodeTemplate
	err := ns.db.DB.Where("node_id = ?", req.NodeID).First(&existingNode).Error
	if err == nil {
		return nil, fmt.Errorf("node with ID '%s' already exists", req.NodeID)
	}

	node := &models.NodeTemplate{
		NodeID:                req.NodeID,
		Name:                  req.Name,
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

	// Create independent action templates and collect their IDs
	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		// Create action using ActionService
		actionTemplate, err := ns.createActionTemplateInTx(tx, &actionReq)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action template: %w", err)
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	// Set action template IDs in node
	if err := node.SetActionTemplateIDs(actionTemplateIDs); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to set action template IDs: %w", err)
	}

	// Create node
	if err := tx.Create(node).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Node Service] Node created successfully: %s (ID: %d)", node.NodeID, node.ID)

	// Fetch the complete node with actions
	return ns.GetNode(node.ID)
}

func (ns *NodeService) GetNode(nodeID uint) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := ns.db.DB.Where("id = ?", nodeID).First(&node).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	return &node, nil
}

func (ns *NodeService) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	var node models.NodeTemplate
	err := ns.db.DB.Where("node_id = ?", nodeID).First(&node).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	return &node, nil
}

func (ns *NodeService) GetNodeWithActions(nodeID uint) (*NodeWithActions, error) {
	node, err := ns.GetNode(nodeID)
	if err != nil {
		return nil, err
	}

	// Get associated action templates
	actionIDs, err := node.GetActionTemplateIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to parse action template IDs: %w", err)
	}

	var actions []models.ActionTemplate
	if len(actionIDs) > 0 {
		err = ns.db.DB.Where("id IN ?", actionIDs).
			Preload("Parameters").
			Find(&actions).Error
		if err != nil {
			return nil, fmt.Errorf("failed to get actions: %w", err)
		}
	}

	return &NodeWithActions{
		NodeTemplate: *node,
		Actions:      actions,
	}, nil
}

func (ns *NodeService) ListNodes(limit, offset int) ([]models.NodeTemplate, error) {
	var nodes []models.NodeTemplate
	query := ns.db.DB.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&nodes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	log.Printf("[Node Service] Retrieved %d nodes", len(nodes))
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
		err := ns.db.DB.Where("node_id = ? AND id != ?",
			req.NodeID, nodeID).First(&conflictNode).Error
		if err == nil {
			return nil, fmt.Errorf("node with ID '%s' already exists", req.NodeID)
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

	// Delete old action templates if they exist
	oldActionIDs, err := existingNode.GetActionTemplateIDs()
	if err == nil && len(oldActionIDs) > 0 {
		// Delete old action templates and their parameters
		if err := ns.deleteActionTemplatesInTx(tx, oldActionIDs); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete old action templates: %w", err)
		}
	}

	// Create new action templates
	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := ns.createActionTemplateInTx(tx, &actionReq)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action template: %w", err)
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	// Update node
	updateNode := &models.NodeTemplate{
		NodeID:                req.NodeID,
		Name:                  req.Name,
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

	// Set new action template IDs
	if err := updateNode.SetActionTemplateIDs(actionTemplateIDs); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to set action template IDs: %w", err)
	}

	if err := tx.Model(&models.NodeTemplate{}).Where("id = ?", nodeID).Updates(updateNode).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update node: %w", err)
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

	// Delete associated action templates
	actionIDs, err := node.GetActionTemplateIDs()
	if err == nil && len(actionIDs) > 0 {
		if err := ns.deleteActionTemplatesInTx(tx, actionIDs); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete action templates: %w", err)
		}
	}

	// Delete template associations
	if err := tx.Where("node_template_id = ?", nodeID).Delete(&models.OrderTemplateNode{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete template associations: %w", err)
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

func (ns *NodeService) createActionTemplateInTx(tx *gorm.DB, actionReq *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
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
		valueStr, err := ns.convertValueToString(paramReq.Value, paramReq.ValueType)
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

func (ns *NodeService) deleteActionTemplatesInTx(tx *gorm.DB, actionIDs []uint) error {
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

// Helper structures are defined in types.go
