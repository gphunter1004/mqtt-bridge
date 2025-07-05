package services

import (
	"encoding/json"
	"fmt"
	"log"

	"mqtt-bridge/database"
	"mqtt-bridge/models"

	"gorm.io/gorm"
)

type EdgeService struct {
	db *database.Database
}

func NewEdgeService(db *database.Database) *EdgeService {
	return &EdgeService{
		db: db,
	}
}

// Edge Management

func (es *EdgeService) CreateEdge(templateID uint, req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	// Check if template exists
	var template models.OrderTemplate
	if err := es.db.DB.First(&template, templateID).Error; err != nil {
		return nil, fmt.Errorf("order template not found: %w", err)
	}

	// Check if edge ID already exists in this template
	var existingEdge models.EdgeTemplate
	err := es.db.DB.Where("order_template_id = ? AND edge_id = ?", templateID, req.EdgeID).First(&existingEdge).Error
	if err == nil {
		return nil, fmt.Errorf("edge with ID '%s' already exists in template %d", req.EdgeID, templateID)
	}

	// Validate that start and end nodes exist
	var startNode, endNode models.NodeTemplate
	if err := es.db.DB.Where("order_template_id = ? AND node_id = ?", templateID, req.StartNodeID).First(&startNode).Error; err != nil {
		return nil, fmt.Errorf("start node '%s' not found in template", req.StartNodeID)
	}
	if err := es.db.DB.Where("order_template_id = ? AND node_id = ?", templateID, req.EndNodeID).First(&endNode).Error; err != nil {
		return nil, fmt.Errorf("end node '%s' not found in template", req.EndNodeID)
	}

	edge := &models.EdgeTemplate{
		OrderTemplateID: templateID,
		EdgeID:          req.EdgeID,
		SequenceID:      req.SequenceID,
		Released:        req.Released,
		StartNodeID:     req.StartNodeID,
		EndNodeID:       req.EndNodeID,
	}

	// Start transaction
	tx := es.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create edge
	if err := tx.Create(edge).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create edge: %w", err)
	}

	// Create actions for edge
	for _, actionReq := range req.Actions {
		if err := es.createActionTemplate(tx, &actionReq, nil, &edge.ID); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create edge action: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Edge Service] Edge created successfully: %s (ID: %d) in template %d", edge.EdgeID, edge.ID, templateID)

	// Fetch the complete edge with actions
	return es.GetEdge(edge.ID)
}

func (es *EdgeService) GetEdge(edgeID uint) (*models.EdgeTemplate, error) {
	var edge models.EdgeTemplate
	err := es.db.DB.Where("id = ?", edgeID).
		Preload("Actions.Parameters").
		First(&edge).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	return &edge, nil
}

func (es *EdgeService) GetEdgeByEdgeID(templateID uint, edgeID string) (*models.EdgeTemplate, error) {
	var edge models.EdgeTemplate
	err := es.db.DB.Where("order_template_id = ? AND edge_id = ?", templateID, edgeID).
		Preload("Actions.Parameters").
		First(&edge).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	return &edge, nil
}

func (es *EdgeService) ListEdges(templateID uint) ([]models.EdgeTemplate, error) {
	var edges []models.EdgeTemplate
	err := es.db.DB.Where("order_template_id = ?", templateID).
		Preload("Actions.Parameters").
		Order("sequence_id ASC").
		Find(&edges).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list edges: %w", err)
	}

	log.Printf("[Edge Service] Retrieved %d edges for template %d", len(edges), templateID)
	return edges, nil
}

func (es *EdgeService) UpdateEdge(edgeID uint, req *models.EdgeTemplateRequest) (*models.EdgeTemplate, error) {
	// Get existing edge
	existingEdge, err := es.GetEdge(edgeID)
	if err != nil {
		return nil, fmt.Errorf("edge not found: %w", err)
	}

	// Check for edgeID conflicts (if edgeID is being changed)
	if existingEdge.EdgeID != req.EdgeID {
		var conflictEdge models.EdgeTemplate
		err := es.db.DB.Where("order_template_id = ? AND edge_id = ? AND id != ?",
			existingEdge.OrderTemplateID, req.EdgeID, edgeID).First(&conflictEdge).Error
		if err == nil {
			return nil, fmt.Errorf("edge with ID '%s' already exists in template", req.EdgeID)
		}
	}

	// Validate that start and end nodes exist
	var startNode, endNode models.NodeTemplate
	if err := es.db.DB.Where("order_template_id = ? AND node_id = ?", existingEdge.OrderTemplateID, req.StartNodeID).First(&startNode).Error; err != nil {
		return nil, fmt.Errorf("start node '%s' not found in template", req.StartNodeID)
	}
	if err := es.db.DB.Where("order_template_id = ? AND node_id = ?", existingEdge.OrderTemplateID, req.EndNodeID).First(&endNode).Error; err != nil {
		return nil, fmt.Errorf("end node '%s' not found in template", req.EndNodeID)
	}

	// Start transaction
	tx := es.db.DB.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update edge
	updateEdge := &models.EdgeTemplate{
		EdgeID:      req.EdgeID,
		SequenceID:  req.SequenceID,
		Released:    req.Released,
		StartNodeID: req.StartNodeID,
		EndNodeID:   req.EndNodeID,
	}

	if err := tx.Model(&models.EdgeTemplate{}).Where("id = ?", edgeID).Updates(updateEdge).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update edge: %w", err)
	}

	// Delete existing actions and parameters
	if err := es.deleteEdgeActions(tx, edgeID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete existing actions: %w", err)
	}

	// Create new actions
	for _, actionReq := range req.Actions {
		if err := es.createActionTemplate(tx, &actionReq, nil, &edgeID); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create action: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Edge Service] Edge updated successfully: %s (ID: %d)", req.EdgeID, edgeID)
	return es.GetEdge(edgeID)
}

func (es *EdgeService) DeleteEdge(edgeID uint) error {
	// Get edge info for logging
	edge, err := es.GetEdge(edgeID)
	if err != nil {
		return fmt.Errorf("edge not found: %w", err)
	}

	// Start transaction
	tx := es.db.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete actions and parameters
	if err := es.deleteEdgeActions(tx, edgeID); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete edge actions: %w", err)
	}

	// Delete edge
	if err := tx.Delete(&models.EdgeTemplate{}, edgeID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete edge: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[Edge Service] Edge deleted successfully: %s (ID: %d)", edge.EdgeID, edgeID)
	return nil
}

// Edge validation
func (es *EdgeService) ValidateEdge(edgeID uint) (*EdgeValidationResult, error) {
	edge, err := es.GetEdge(edgeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	result := &EdgeValidationResult{
		EdgeID:      edge.EdgeID,
		IsValid:     true,
		Errors:      []string{},
		Warnings:    []string{},
		ActionCount: len(edge.Actions),
		StartNodeID: edge.StartNodeID,
		EndNodeID:   edge.EndNodeID,
	}

	// Check if start and end nodes exist
	var startNode, endNode models.NodeTemplate
	startNodeExists := es.db.DB.Where("order_template_id = ? AND node_id = ?",
		edge.OrderTemplateID, edge.StartNodeID).First(&startNode).Error == nil
	endNodeExists := es.db.DB.Where("order_template_id = ? AND node_id = ?",
		edge.OrderTemplateID, edge.EndNodeID).First(&endNode).Error == nil

	if !startNodeExists {
		result.Errors = append(result.Errors, fmt.Sprintf("Start node '%s' does not exist", edge.StartNodeID))
		result.IsValid = false
	}
	if !endNodeExists {
		result.Errors = append(result.Errors, fmt.Sprintf("End node '%s' does not exist", edge.EndNodeID))
		result.IsValid = false
	}

	// Check for self-loop
	if edge.StartNodeID == edge.EndNodeID {
		result.Warnings = append(result.Warnings, "Edge creates a self-loop (start and end nodes are the same)")
	}

	// Check for invalid sequence ID
	if edge.SequenceID < 0 {
		result.Errors = append(result.Errors, "Edge sequence ID cannot be negative")
		result.IsValid = false
	}

	// Validate actions
	for i, action := range edge.Actions {
		if action.ActionType == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Action %d has no action type", i+1))
			result.IsValid = false
		}
		if action.ActionID == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Action %d has no action ID", i+1))
		}
	}

	return result, nil
}

// Get edges connecting specific nodes
func (es *EdgeService) GetEdgesBetweenNodes(templateID uint, startNodeID, endNodeID string) ([]models.EdgeTemplate, error) {
	var edges []models.EdgeTemplate
	query := es.db.DB.Where("order_template_id = ?", templateID).
		Preload("Actions.Parameters")

	if startNodeID != "" && endNodeID != "" {
		// Get edges between specific nodes
		query = query.Where("start_node_id = ? AND end_node_id = ?", startNodeID, endNodeID)
	} else if startNodeID != "" {
		// Get edges starting from specific node
		query = query.Where("start_node_id = ?", startNodeID)
	} else if endNodeID != "" {
		// Get edges ending at specific node
		query = query.Where("end_node_id = ?", endNodeID)
	}

	err := query.Find(&edges).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get edges: %w", err)
	}

	return edges, nil
}

// Get all edges connected to a specific node
func (es *EdgeService) GetConnectedEdges(templateID uint, nodeID string) (*ConnectedEdgesResult, error) {
	incomingEdges, err := es.GetEdgesBetweenNodes(templateID, "", nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming edges: %w", err)
	}

	outgoingEdges, err := es.GetEdgesBetweenNodes(templateID, nodeID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get outgoing edges: %w", err)
	}

	result := &ConnectedEdgesResult{
		NodeID:        nodeID,
		IncomingEdges: incomingEdges,
		OutgoingEdges: outgoingEdges,
		TotalEdges:    len(incomingEdges) + len(outgoingEdges),
	}

	return result, nil
}

// Check for cycles in the edge graph
func (es *EdgeService) CheckForCycles(templateID uint) (*CycleCheckResult, error) {
	edges, err := es.ListEdges(templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges: %w", err)
	}

	// Build adjacency list
	graph := make(map[string][]string)
	for _, edge := range edges {
		graph[edge.StartNodeID] = append(graph[edge.StartNodeID], edge.EndNodeID)
	}

	// DFS-based cycle detection
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	cycles := [][]string{}

	var dfs func(string, []string) bool
	dfs = func(node string, path []string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, neighbor := range graph[node] {
			if !visited[neighbor] {
				if dfs(neighbor, path) {
					return true
				}
			} else if recStack[neighbor] {
				// Found a cycle
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart != -1 {
					cycle := append(path[cycleStart:], neighbor)
					cycles = append(cycles, cycle)
				}
				return true
			}
		}

		recStack[node] = false
		return false
	}

	// Check all nodes
	for _, edge := range edges {
		if !visited[edge.StartNodeID] {
			dfs(edge.StartNodeID, []string{})
		}
	}

	result := &CycleCheckResult{
		HasCycles:  len(cycles) > 0,
		CycleCount: len(cycles),
		Cycles:     cycles,
	}

	return result, nil
}

// Helper functions

func (es *EdgeService) createActionTemplate(tx *gorm.DB, actionReq *models.ActionTemplateRequest,
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
		valueStr, err := es.convertValueToString(paramReq.Value, paramReq.ValueType)
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

func (es *EdgeService) convertValueToString(value interface{}, valueType string) (string, error) {
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

func (es *EdgeService) deleteEdgeActions(tx *gorm.DB, edgeID uint) error {
	// Delete action parameters
	if err := tx.Exec(`
		DELETE FROM action_parameter_templates 
		WHERE action_template_id IN (
			SELECT id FROM action_templates WHERE edge_template_id = ?
		)
	`, edgeID).Error; err != nil {
		return err
	}

	// Delete actions
	if err := tx.Where("edge_template_id = ?", edgeID).Delete(&models.ActionTemplate{}).Error; err != nil {
		return err
	}

	return nil
}

// Response structures
type EdgeValidationResult struct {
	EdgeID      string   `json:"edgeId"`
	IsValid     bool     `json:"isValid"`
	Errors      []string `json:"errors"`
	Warnings    []string `json:"warnings"`
	ActionCount int      `json:"actionCount"`
	StartNodeID string   `json:"startNodeId"`
	EndNodeID   string   `json:"endNodeId"`
}

type ConnectedEdgesResult struct {
	NodeID        string                `json:"nodeId"`
	IncomingEdges []models.EdgeTemplate `json:"incomingEdges"`
	OutgoingEdges []models.EdgeTemplate `json:"outgoingEdges"`
	TotalEdges    int                   `json:"totalEdges"`
}

type CycleCheckResult struct {
	HasCycles  bool       `json:"hasCycles"`
	CycleCount int        `json:"cycleCount"`
	Cycles     [][]string `json:"cycles"`
}
