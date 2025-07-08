package services

import (
	"fmt"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
)

type EdgeService struct {
	edgeRepo      interfaces.EdgeRepositoryInterface
	actionService *ActionService // Changed from actionRepo
}

func NewEdgeService(edgeRepo interfaces.EdgeRepositoryInterface, actionService *ActionService) *EdgeService {
	return &EdgeService{
		edgeRepo:      edgeRepo,
		actionService: actionService,
	}
}

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

	// Create action templates and collect their IDs
	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := es.actionService.CreateActionTemplate(&actionReq)
		if err != nil {
			// Log error but continue with other actions
			continue
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	// Set action template IDs in edge
	if len(actionTemplateIDs) > 0 {
		if err := edge.SetActionTemplateIDs(actionTemplateIDs); err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
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

	// Delete old action templates
	newActionIDs, err := es.actionService.RecreateActionTemplatesForOwner(existingEdge.ActionTemplateIDs, req.Actions)
	if err != nil {
		return nil, fmt.Errorf("failed to update action templates for edge: %w", err)
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

	// Set new action template IDs
	if len(newActionIDs) > 0 {
		if err := edge.SetActionTemplateIDs(newActionIDs); err != nil {
			return nil, fmt.Errorf("failed to set action template IDs: %w", err)
		}
	}

	// Update edge using repository
	return es.edgeRepo.UpdateEdge(edgeID, edge)
}

func (es *EdgeService) DeleteEdge(edgeID uint) error {
	return es.edgeRepo.DeleteEdge(edgeID)
}
