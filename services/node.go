package services

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
)

// NodeWithActions represents a node template with its associated actions
type NodeWithActions struct {
	NodeTemplate models.NodeTemplate     `json:"nodeTemplate"`
	Actions      []models.ActionTemplate `json:"actions"`
}

type NodeService struct {
	nodeRepo      interfaces.NodeRepositoryInterface
	actionService *ActionService // Changed from actionRepo
}

func NewNodeService(nodeRepo interfaces.NodeRepositoryInterface, actionService *ActionService) *NodeService {
	return &NodeService{
		nodeRepo:      nodeRepo,
		actionService: actionService,
	}
}

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

	// Create action templates and collect their IDs
	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := ns.actionService.CreateActionTemplate(&actionReq)
		if err != nil {
			// Log error but continue with other actions
			continue
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	// Set action template IDs in node
	if len(actionTemplateIDs) > 0 {
		if err := node.SetActionTemplateIDs(actionTemplateIDs); err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
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

func (ns *NodeService) GetNodeWithActions(nodeID uint) (*NodeWithActions, error) {
	// Get node and actions using repository
	node, actions, err := ns.nodeRepo.GetNodeWithActions(nodeID)
	if err != nil {
		return nil, err
	}

	return &NodeWithActions{
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

	// Delete old action templates
	newActionIDs, err := ns.actionService.RecreateActionTemplatesForOwner(existingNode.ActionTemplateIDs, req.Actions)
	if err != nil {
		return nil, fmt.Errorf("failed to update action templates for node: %w", err)
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

	// Set new action template IDs
	if len(newActionIDs) > 0 {
		if err := node.SetActionTemplateIDs(newActionIDs); err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
	}

	// Update node using repository
	return ns.nodeRepo.UpdateNode(nodeID, node)
}

func (ns *NodeService) DeleteNode(nodeID uint) error {
	return ns.nodeRepo.DeleteNode(nodeID)
}
