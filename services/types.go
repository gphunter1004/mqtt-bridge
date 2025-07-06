package services

import "mqtt-bridge/models"

// Common types shared across services

// NodeWithActions represents a node template with its associated actions
type NodeWithActions struct {
	NodeTemplate models.NodeTemplate     `json:"nodeTemplate"`
	Actions      []models.ActionTemplate `json:"actions"`
}

// EdgeWithActions represents an edge template with its associated actions
type EdgeWithActions struct {
	EdgeTemplate models.EdgeTemplate     `json:"edgeTemplate"`
	Actions      []models.ActionTemplate `json:"actions"`
}

// OrderTemplateWithDetails represents an order template with detailed node and edge information
type OrderTemplateWithDetails struct {
	OrderTemplate    models.OrderTemplate `json:"orderTemplate"`
	NodesWithActions []NodeWithActions    `json:"nodesWithActions"`
	EdgesWithActions []EdgeWithActions    `json:"edgesWithActions"`
}
