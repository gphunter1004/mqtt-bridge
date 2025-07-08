package services

import (
	"fmt"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"
)

// NodeWithActions is a DTO that combines a node template with its fully populated actions.
type NodeWithActions struct {
	NodeTemplate models.NodeTemplate     `json:"nodeTemplate"`
	Actions      []models.ActionTemplate `json:"actions"`
}

// NodeService handles business logic related to node templates.
type NodeService struct {
	nodeRepo      interfaces.NodeRepositoryInterface
	actionService *ActionService
	uow           database.UnitOfWorkInterface
}

// NewNodeService creates a new instance of NodeService.
func NewNodeService(
	nodeRepo interfaces.NodeRepositoryInterface,
	actionService *ActionService,
	uow database.UnitOfWorkInterface,
) *NodeService {
	return &NodeService{
		nodeRepo:      nodeRepo,
		actionService: actionService,
		uow:           uow,
	}
}

// CreateNode creates a new node template along with its associated actions within a single transaction.
func (ns *NodeService) CreateNode(req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	exists, err := ns.nodeRepo.CheckNodeExists(req.NodeID)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to check for existing node.", err)
	}
	if exists {
		return nil, utils.NewBadRequestError(fmt.Sprintf("Node with ID '%s' already exists.", req.NodeID))
	}

	tx := ns.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			ns.uow.Rollback(tx)
			panic(r)
		}
	}()

	var actionTemplateIDs []uint
	if len(req.Actions) > 0 {
		// Since this is a creation, actions are created within the same transaction.
		actionTemplateIDs, err = ns.actionService.RecreateActionTemplatesForOwner(tx, "", req.Actions)
		if err != nil {
			ns.uow.Rollback(tx)
			return nil, utils.NewInternalServerError("Failed to create action templates for node.", err)
		}
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

	if len(actionTemplateIDs) > 0 {
		if err := node.SetActionTemplateIDs(actionTemplateIDs); err != nil {
			ns.uow.Rollback(tx)
			return nil, utils.NewInternalServerError("Failed to set action template IDs on node.", err)
		}
	}

	createdNode, err := ns.nodeRepo.CreateNode(tx, node)
	if err != nil {
		ns.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to create node in repository.", err)
	}

	if err := ns.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit transaction for node creation.", err)
	}

	return createdNode, nil
}

// GetNode retrieves a single node template by its database ID.
func (ns *NodeService) GetNode(nodeID uint) (*models.NodeTemplate, error) {
	node, err := ns.nodeRepo.GetNode(nodeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Node with ID %d not found.", nodeID))
	}
	return node, nil
}

// GetNodeByNodeID retrieves a single node template by its string ID.
func (ns *NodeService) GetNodeByNodeID(nodeID string) (*models.NodeTemplate, error) {
	node, err := ns.nodeRepo.GetNodeByNodeID(nodeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Node with nodeId '%s' not found.", nodeID))
	}
	return node, nil
}

// GetNodeWithActions retrieves a node and its fully populated action templates.
func (ns *NodeService) GetNodeWithActions(nodeID uint) (*NodeWithActions, error) {
	node, actions, err := ns.nodeRepo.GetNodeWithActions(nodeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Node with ID %d not found.", nodeID))
	}
	return &NodeWithActions{NodeTemplate: *node, Actions: actions}, nil
}

// ListNodes retrieves a paginated list of all node templates.
func (ns *NodeService) ListNodes(limit, offset int) ([]models.NodeTemplate, error) {
	nodes, err := ns.nodeRepo.ListNodes(limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to list nodes.", err)
	}
	return nodes, nil
}

// UpdateNode updates an existing node template within a single transaction.
func (ns *NodeService) UpdateNode(nodeID uint, req *models.NodeTemplateRequest) (*models.NodeTemplate, error) {
	existingNode, err := ns.nodeRepo.GetNode(nodeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Node with ID %d not found for update.", nodeID))
	}

	if existingNode.NodeID != req.NodeID {
		exists, err := ns.nodeRepo.CheckNodeExistsExcluding(req.NodeID, nodeID)
		if err != nil {
			return nil, utils.NewInternalServerError("Failed to check for node ID conflict.", err)
		}
		if exists {
			return nil, utils.NewBadRequestError(fmt.Sprintf("Node with ID '%s' already exists.", req.NodeID))
		}
	}

	tx := ns.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			ns.uow.Rollback(tx)
			panic(r)
		}
	}()

	// Pass the transaction `tx` to the action service ---
	newActionIDs, err := ns.actionService.RecreateActionTemplatesForOwner(tx, existingNode.ActionTemplateIDs, req.Actions)
	if err != nil {
		ns.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to update action templates for node.", err)
	}

	nodeToUpdate := &models.NodeTemplate{
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

	if err := nodeToUpdate.SetActionTemplateIDs(newActionIDs); err != nil {
		ns.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to set new action template IDs on node.", err)
	}

	updatedNode, err := ns.nodeRepo.UpdateNode(tx, nodeID, nodeToUpdate)
	if err != nil {
		ns.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to update node in repository.", err)
	}

	if err := ns.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit transaction for node update.", err)
	}

	return updatedNode, nil
}

// DeleteNode deletes a node template and its associations within a single transaction.
func (ns *NodeService) DeleteNode(nodeID uint) error {
	tx := ns.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			ns.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := ns.nodeRepo.DeleteNode(tx, nodeID); err != nil {
		ns.uow.Rollback(tx)
		return utils.NewInternalServerError(fmt.Sprintf("Failed to delete node with ID %d.", nodeID), err)
	}

	if err := ns.uow.Commit(tx); err != nil {
		return utils.NewInternalServerError("Failed to commit transaction for node deletion.", err)
	}

	return nil
}
