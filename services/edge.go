package services

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/services/base"
	"mqtt-bridge/utils"
)

type EdgeService struct {
	edgeRepo              interfaces.EdgeRepositoryInterface
	actionTemplateManager *base.ActionTemplateManager
}

func NewEdgeService(edgeRepo interfaces.EdgeRepositoryInterface, actionRepo interfaces.ActionRepositoryInterface) *EdgeService {
	return &EdgeService{
		edgeRepo:              edgeRepo,
		actionTemplateManager: base.NewActionTemplateManager(actionRepo),
	}
}

// ===================================================================
// EDGE CRUD OPERATIONS
// ===================================================================

func (es *EdgeService) CreateEdge(req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	// Check if edge with this edgeID already exists
	exists, err := es.edgeRepo.CheckEdgeExists(req.EdgeID)
	if err != nil {
		return nil, fmt.Errorf("failed to check edge existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("edge with ID '%s' already exists", req.EdgeID)
	}

	// Prepare edge template
	edge := &models.EdgeTemplate{
		EdgeID:      req.EdgeID,
		Name:        req.Name,
		Description: req.Description,
		SequenceID:  req.SequenceID,
		Released:    req.Released,
		StartNodeID: req.StartNodeID,
		EndNodeID:   req.EndNodeID,
	}

	// Create action templates using the common manager
	actionTemplateIDs, err := es.actionTemplateManager.CreateActionTemplates(req.Actions)
	if err != nil {
		utils.LogError(utils.LogComponentService, "Failed to create some action templates for edge %s", req.EdgeID)
	}

	// Set action template IDs in edge using utils helper
	if len(actionTemplateIDs) > 0 {
		actionIDsJSON, err := utils.ConvertUintSliceToJSON(actionTemplateIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
		edge.ActionTemplateIDs = actionIDsJSON
	}

	// Create edge using repository
	return es.edgeRepo.CreateEdge(edge)
}

func (es *EdgeService) GetEdge(edgeID uint) (*models.EdgeTemplate, error) {
	return es.edgeRepo.GetEdge(edgeID)
}

func (es *EdgeService) GetEdgeByEdgeID(edgeID string) (*models.EdgeTemplate, error) {
	return es.edgeRepo.GetEdgeByEdgeID(edgeID)
}

func (es *EdgeService) GetEdgeWithActions(edgeID uint) (*models.EdgeWithActions, error) {
	// Get edge and actions using repository
	edge, actions, err := es.edgeRepo.GetEdgeWithActions(edgeID)
	if err != nil {
		return nil, err
	}

	return &models.EdgeWithActions{
		EdgeTemplate: *edge,
		Actions:      actions,
	}, nil
}

func (es *EdgeService) ListEdges(limit, offset int) ([]models.EdgeTemplate, error) {
	return es.edgeRepo.ListEdges(limit, offset)
}

func (es *EdgeService) UpdateEdge(edgeID uint, req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	// Check if edge exists
	existingEdge, err := es.edgeRepo.GetEdge(edgeID)
	if err != nil {
		return nil, fmt.Errorf("edge not found: %w", err)
	}

	// Check for edgeID conflicts (if edgeID is changing)
	if existingEdge.EdgeID != req.EdgeID {
		exists, err := es.edgeRepo.CheckEdgeExistsExcluding(req.EdgeID, edgeID)
		if err != nil {
			return nil, fmt.Errorf("failed to check edge ID conflict: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("edge with ID '%s' already exists", req.EdgeID)
		}
	}

	// Delete old action templates using the common manager
	if existingEdge.ActionTemplateIDs != "" {
		oldActionIDs, err := utils.ParseJSONToUintSlice(existingEdge.ActionTemplateIDs)
		if err == nil && len(oldActionIDs) > 0 {
			es.actionTemplateManager.DeleteActionTemplates(oldActionIDs)
		}
	}

	// Prepare updated edge template
	edge := &models.EdgeTemplate{
		EdgeID:      req.EdgeID,
		Name:        req.Name,
		Description: req.Description,
		SequenceID:  req.SequenceID,
		Released:    req.Released,
		StartNodeID: req.StartNodeID,
		EndNodeID:   req.EndNodeID,
	}

	// Create new action templates using the common manager
	actionTemplateIDs, err := es.actionTemplateManager.CreateActionTemplates(req.Actions)
	if err != nil {
		utils.LogError(utils.LogComponentService, "Failed to create some action templates during edge update")
	}

	// Set new action template IDs using utils helper
	if len(actionTemplateIDs) > 0 {
		actionIDsJSON, err := utils.ConvertUintSliceToJSON(actionTemplateIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
		edge.ActionTemplateIDs = actionIDsJSON
	}

	// Update edge using repository
	return es.edgeRepo.UpdateEdge(edgeID, edge)
}

func (es *EdgeService) DeleteEdge(edgeID uint) error {
	return es.edgeRepo.DeleteEdge(edgeID)
}
