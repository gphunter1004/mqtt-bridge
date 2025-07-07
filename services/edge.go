package services

import (
	"fmt"

	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/utils"
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

func (es *EdgeService) CreateEdge(req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
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

	if err := es.db.DB.Create(edge).Error; err != nil {
		return nil, fmt.Errorf("failed to create edge: %w", err)
	}

	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := es.createActionTemplate(&actionReq)
		if err != nil {
			continue
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	if len(actionTemplateIDs) > 0 {
		edge.SetActionTemplateIDs(actionTemplateIDs)
		es.db.DB.Save(edge)
	}

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

	return edges, nil
}

func (es *EdgeService) UpdateEdge(edgeID uint, req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	existingEdge, err := es.GetEdge(edgeID)
	if err != nil {
		return nil, fmt.Errorf("edge not found: %w", err)
	}

	if existingEdge.EdgeID != req.EdgeID {
		var conflictEdge models.EdgeTemplate
		err := es.db.DB.Where("edge_id = ? AND id != ?",
			req.EdgeID, edgeID).First(&conflictEdge).Error
		if err == nil {
			return nil, fmt.Errorf("edge with ID '%s' already exists", req.EdgeID)
		}
	}

	oldActionIDs, err := existingEdge.GetActionTemplateIDs()
	if err == nil && len(oldActionIDs) > 0 {
		es.deleteActionTemplates(oldActionIDs)
	}

	updateEdge := &models.EdgeTemplate{
		EdgeID:      req.EdgeID,
		Name:        req.Name,
		Description: req.Description,
		SequenceID:  req.SequenceID,
		Released:    req.Released,
		StartNodeID: req.StartNodeID,
		EndNodeID:   req.EndNodeID,
	}

	if err := es.db.DB.Model(&models.EdgeTemplate{}).Where("id = ?", edgeID).Updates(updateEdge).Error; err != nil {
		return nil, fmt.Errorf("failed to update edge: %w", err)
	}

	var actionTemplateIDs []uint
	for _, actionReq := range req.Actions {
		actionTemplate, err := es.createActionTemplate(&actionReq)
		if err != nil {
			continue
		}
		actionTemplateIDs = append(actionTemplateIDs, actionTemplate.ID)
	}

	if len(actionTemplateIDs) > 0 {
		updateEdge.SetActionTemplateIDs(actionTemplateIDs)
		es.db.DB.Model(&models.EdgeTemplate{}).Where("id = ?", edgeID).Update("action_template_ids", updateEdge.ActionTemplateIDs)
	}

	return es.GetEdge(edgeID)
}

func (es *EdgeService) DeleteEdge(edgeID uint) error {
	edge, err := es.GetEdge(edgeID)
	if err != nil {
		return fmt.Errorf("edge not found: %w", err)
	}

	actionIDs, err := edge.GetActionTemplateIDs()
	if err == nil && len(actionIDs) > 0 {
		es.deleteActionTemplates(actionIDs)
	}

	es.db.DB.Where("edge_template_id = ?", edgeID).Delete(&models.OrderTemplateEdge{})

	if err := es.db.DB.Delete(&models.EdgeTemplate{}, edgeID).Error; err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}

	return nil
}

func (es *EdgeService) createActionTemplate(actionReq *models.ActionTemplateRequest) (*models.ActionTemplate, error) {
	action := &models.ActionTemplate{
		ActionType:        actionReq.ActionType,
		ActionID:          actionReq.ActionID,
		BlockingType:      actionReq.BlockingType,
		ActionDescription: actionReq.ActionDescription,
	}

	if err := es.db.DB.Create(action).Error; err != nil {
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

		es.db.DB.Create(param)
	}

	return action, nil
}

func (es *EdgeService) deleteActionTemplates(actionIDs []uint) {
	es.db.DB.Where("action_template_id IN ?", actionIDs).Delete(&models.ActionParameterTemplate{})
	es.db.DB.Where("id IN ?", actionIDs).Delete(&models.ActionTemplate{})
}
