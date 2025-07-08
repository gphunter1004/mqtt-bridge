package services

import (
	"fmt"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"
)

// EdgeService handles business logic related to edge templates.
type EdgeService struct {
	edgeRepo      interfaces.EdgeRepositoryInterface
	actionService *ActionService
	uow           database.UnitOfWorkInterface
}

// NewEdgeService creates a new instance of EdgeService.
func NewEdgeService(
	edgeRepo interfaces.EdgeRepositoryInterface,
	actionService *ActionService,
	uow database.UnitOfWorkInterface,
) *EdgeService {
	return &EdgeService{
		edgeRepo:      edgeRepo,
		actionService: actionService,
		uow:           uow,
	}
}

// CreateEdge creates a new edge template along with its associated actions within a single transaction.
func (es *EdgeService) CreateEdge(req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	exists, err := es.edgeRepo.CheckEdgeExists(req.EdgeID)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to check for existing edge.", err)
	}
	if exists {
		return nil, utils.NewBadRequestError(fmt.Sprintf("Edge with ID '%s' already exists.", req.EdgeID))
	}

	tx := es.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			es.uow.Rollback(tx)
			panic(r)
		}
	}()

	var actionTemplateIDs []uint
	if len(req.Actions) > 0 {
		actionTemplateIDs, err = es.actionService.RecreateActionTemplatesForOwner(tx, "", req.Actions)
		if err != nil {
			es.uow.Rollback(tx)
			return nil, utils.NewInternalServerError("Failed to create action templates for edge.", err)
		}
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

	if err := edge.SetActionTemplateIDs(actionTemplateIDs); err != nil {
		es.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to set action template IDs on edge.", err)
	}

	createdEdge, err := es.edgeRepo.CreateEdge(tx, edge)
	if err != nil {
		es.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to create edge in repository.", err)
	}

	if err := es.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit transaction for edge creation.", err)
	}

	return createdEdge, nil
}

// GetEdge retrieves a single edge template by its database ID.
func (es *EdgeService) GetEdge(edgeID uint) (*models.EdgeTemplate, error) {
	edge, err := es.edgeRepo.GetEdge(edgeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Edge with ID %d not found.", edgeID))
	}
	return edge, nil
}

// GetEdgeByEdgeID retrieves a single edge template by its string ID.
func (es *EdgeService) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	edge, err := es.edgeRepo.GetEdgeByEdgeID(edgeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Edge with edgeId '%s' not found.", edgeID))
	}
	return edge, nil
}

// GetEdgeWithActions retrieves an edge and its fully populated action templates.
func (es *EdgeService) GetEdgeWithActions(edgeID uint) (*models.EdgeWithActions, error) {
	edge, actions, err := es.edgeRepo.GetEdgeWithActions(edgeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Edge with ID %d not found.", edgeID))
	}
	return &models.EdgeWithActions{EdgeTemplate: *edge, Actions: actions}, nil
}

// ListEdges retrieves a paginated list of all edge templates.
func (es *EdgeService) ListEdges(limit, offset int) ([]models.EdgeTemplate, error) {
	edges, err := es.edgeRepo.ListEdges(limit, offset)
	if err != nil {
		return nil, utils.NewInternalServerError("Failed to list edges.", err)
	}
	return edges, nil
}

// UpdateEdge updates an existing edge template within a single transaction.
func (es *EdgeService) UpdateEdge(edgeID uint, req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	existingEdge, err := es.edgeRepo.GetEdge(edgeID)
	if err != nil {
		return nil, utils.NewNotFoundError(fmt.Sprintf("Edge with ID %d not found for update.", edgeID))
	}

	if existingEdge.EdgeID != req.EdgeID {
		exists, err := es.edgeRepo.CheckEdgeExistsExcluding(req.EdgeID, edgeID)
		if err != nil {
			return nil, utils.NewInternalServerError("Failed to check for edge ID conflict.", err)
		}
		if exists {
			return nil, utils.NewBadRequestError(fmt.Sprintf("Edge with ID '%s' already exists.", req.EdgeID))
		}
	}

	tx := es.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			es.uow.Rollback(tx)
			panic(r)
		}
	}()

	// --- (FIXED) Pass the transaction `tx` to the action service ---
	newActionIDs, err := es.actionService.RecreateActionTemplatesForOwner(tx, existingEdge.ActionTemplateIDs, req.Actions)
	if err != nil {
		es.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to update action templates for edge.", err)
	}
	// --- END OF FIX ---

	edgeToUpdate := &models.EdgeTemplate{
		EdgeID:      req.EdgeID,
		Name:        req.Name,
		Description: req.Description,
		SequenceID:  req.SequenceID,
		Released:    req.Released,
		StartNodeID: req.StartNodeID,
		EndNodeID:   req.EndNodeID,
	}

	if err := edgeToUpdate.SetActionTemplateIDs(newActionIDs); err != nil {
		es.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to set new action template IDs on edge.", err)
	}

	updatedEdge, err := es.edgeRepo.UpdateEdge(tx, edgeID, edgeToUpdate)
	if err != nil {
		es.uow.Rollback(tx)
		return nil, utils.NewInternalServerError("Failed to update edge in repository.", err)
	}

	if err := es.uow.Commit(tx); err != nil {
		return nil, utils.NewInternalServerError("Failed to commit transaction for edge update.", err)
	}

	return updatedEdge, nil
}

// DeleteEdge deletes an edge template and its associations within a single transaction.
func (es *EdgeService) DeleteEdge(edgeID uint) error {
	tx := es.uow.Begin()
	defer func() {
		if r := recover(); r != nil {
			es.uow.Rollback(tx)
			panic(r)
		}
	}()

	if err := es.edgeRepo.DeleteEdge(tx, edgeID); err != nil {
		es.uow.Rollback(tx)
		return utils.NewInternalServerError(fmt.Sprintf("Failed to delete edge with ID %d.", edgeID), err)
	}

	if err := es.uow.Commit(tx); err != nil {
		return utils.NewInternalServerError("Failed to commit transaction for edge deletion.", err)
	}

	return nil
}
