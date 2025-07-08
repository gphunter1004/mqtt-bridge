package services

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/services/base"
	"mqtt-bridge/utils"
)

type NodeService struct {
	nodeRepo              interfaces.NodeRepositoryInterface
	actionTemplateManager *base.ActionTemplateManager
}

func NewNodeService(nodeRepo interfaces.NodeRepositoryInterface, actionRepo interfaces.ActionRepositoryInterface) *NodeService {
	return &NodeService{
		nodeRepo:              nodeRepo,
		actionTemplateManager: base.NewActionTemplateManager(actionRepo),
	}
}

// ===================================================================
// NODE CRUD OPERATIONS
// ===================================================================

func (ns *NodeService) CreateNode(req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	// Check if node with this nodeID already exists
	exists, err := ns.nodeRepo.CheckNodeExists(req.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to check node existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("node with ID '%s' already exists", req.NodeID)
	}

	// Prepare node template
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

	// Create action templates using the common manager
	actionTemplateIDs, err := ns.actionTemplateManager.CreateActionTemplates(req.Actions)
	if err != nil {
		utils.LogError(utils.LogComponentService, "Failed to create some action templates for node %s", req.NodeID)
	}

	// Set action template IDs in node using utils helper
	if len(actionTemplateIDs) > 0 {
		actionIDsJSON, err := utils.ConvertUintSliceToJSON(actionTemplateIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
		node.ActionTemplateIDs = actionIDsJSON
	}

	// Create node using repository
	return ns.nodeRepo.CreateNode(node)
}

func (ns *NodeService) GetNode(nodeID uint) (*models.NodeTemplate, error) {
	return ns.nodeRepo.GetNode(nodeID)
}

func (ns *NodeService) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	return ns.nodeRepo.GetNodeByNodeID(nodeID)
}

func (ns *NodeService) GetNodeWithActions(nodeID uint) (*models.NodeWithActions, error) {
	// Get node and actions using repository
	node, actions, err := ns.nodeRepo.GetNodeWithActions(nodeID)
	if err != nil {
		return nil, err
	}

	return &models.NodeWithActions{
		NodeTemplate: *node,
		Actions:      actions,
	}, nil
}

func (ns *NodeService) ListNodes(limit, offset int) ([]models.NodeTemplate, error) {
	return ns.nodeRepo.ListNodes(limit, offset)
}

func (ns *NodeService) UpdateNode(nodeID uint, req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	// Check if node exists
	existingNode, err := ns.nodeRepo.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	// Check for nodeID conflicts (if nodeID is changing)
	if existingNode.NodeID != req.NodeID {
		exists, err := ns.nodeRepo.CheckNodeExistsExcluding(req.NodeID, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to check node ID conflict: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("node with ID '%s' already exists", req.NodeID)
		}
	}

	// Get old action IDs and create new ones using the common manager
	var oldActionIDs []uint
	if existingNode.ActionTemplateIDs != "" {
		oldActionIDs, _ = utils.ParseJSONToUintSlice(existingNode.ActionTemplateIDs)
	}

	// Update action templates using the common manager
	actionTemplateIDs, err := ns.actionTemplateManager.UpdateActionTemplatesFromRequests(oldActionIDs, req.Actions)
	if err != nil {
		utils.LogError(utils.LogComponentService, "Failed to update action templates for node %s", req.NodeID)
	}

	// Prepare updated node template
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

	// Set new action template IDs using utils helper
	if len(actionTemplateIDs) > 0 {
		actionIDsJSON, err := utils.ConvertUintSliceToJSON(actionTemplateIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
		node.ActionTemplateIDs = actionIDsJSON
	}

	// Update node using repository
	return ns.nodeRepo.UpdateNode(nodeID, node)
}

func (ns *NodeService) DeleteNode(nodeID uint) error {
	return ns.nodeRepo.DeleteNode(nodeID)
}
