package services

import (
	"fmt"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/utils"
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

func (ns *NodeService) CreateNode(req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
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

	if err := ns.db.DB.Create(node).Error; err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := ns.createActionTemplate(&actionReq)
		if err != nil {
			continue
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	if len(actionTemplateIDs) > 0 {
		node.SetActionTemplateIDs(actionTemplateIDs)
		ns.db.DB.Save(node)
	}

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

	return nodes, nil
}

func (ns *NodeService) UpdateNode(nodeID uint, req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	existingNode, err := ns.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	if existingNode.NodeID != req.NodeID {
		var conflictNode models.NodeTemplate
		err := ns.db.DB.Where("node_id = ? AND id != ?",
			req.NodeID, nodeID).First(&conflictNode).Error
		if err == nil {
			return nil, fmt.Errorf("node with ID '%s' already exists", req.NodeID)
		}
	}

	oldActionIDs, err := existingNode.GetActionTemplateIDs()
	if err == nil && len(oldActionIDs) > 0 {
		ns.deleteActionTemplates(oldActionIDs)
	}

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

	if err := ns.db.DB.Model(&models.NodeTemplate{}).Where("id = ?", nodeID).Updates(updateNode).Error; err != nil {
		return nil, fmt.Errorf("failed to update node: %w", err)
	}

	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := ns.createActionTemplate(&actionReq)
		if err != nil {
			continue
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	if len(actionTemplateIDs) > 0 {
		updateNode.SetActionTemplateIDs(actionTemplateIDs)
		ns.db.DB.Model(&models.NodeTemplate{}).Where("id = ?", nodeID).Update("action_template_ids", updateNode.ActionTemplateIDs)
	}

	return ns.GetNode(nodeID)
}

func (ns *NodeService) DeleteNode(nodeID uint) error {
	node, err := ns.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	actionIDs, err := node.GetActionTemplateIDs()
	if err == nil && len(actionIDs) > 0 {
		ns.deleteActionTemplates(actionIDs)
	}

	ns.db.DB.Where("node_template_id = ?", nodeID).Delete(&models.OrderTemplateNode{})

	if err := ns.db.DB.Delete(&models.NodeTemplate{}, nodeID).Error; err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	return nil
}

func (ns *NodeService) createActionTemplate(actionReq *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	action := &models.ActionTemplate{
		ActionType:        actionReq.ActionType,
		ActionID:          actionReq.ActionID,
		BlockingType:      actionReq.BlockingType,
		ActionDescription: actionReq.ActionDescription,
	}

	if err := ns.db.DB.Create(action).Error; err != nil {
		return nil, fmt.Errorf("failed to create action: %w", err)
	}

	for _, paramReq := range actionReq.Parameters {
		valueStr, err := utils.ConvertValueToString(paramReq.Value, paramReq.ValueType)
		if err != nil {
			continue
		}

		param := &models.ActionParameterTemplate{
			ActionTemplateID: action.ID,
			Key:              paramReq.Key,
			Value:            valueStr,
			ValueType:        paramReq.ValueType,
		}

		ns.db.DB.Create(param)
	}

	return action, nil
}

func (ns *NodeService) deleteActionTemplates(actionIDs []uint) {
	ns.db.DB.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{})
	ns.db.DB.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{})
}
