package models

// NodeWithActions represents a node template with its associated actions
type NodeWithActions struct {
	NodeTemplate NodeTemplate     `json:"nodeTemplate"`
	Actions      []ActionTemplate `json:"actions"`
}

// EdgeWithActions represents an edge template with its associated actions
type EdgeWithActions struct {
	EdgeTemplate EdgeTemplate     `json:"edgeTemplate"`
	Actions      []ActionTemplate `json:"actions"`
}

// OrderTemplateWithDetails represents an order template with detailed node and edge information
type OrderTemplateWithDetails struct {
	OrderTemplate    OrderTemplate     `json:"orderTemplate"`
	NodesWithActions []NodeWithActions `json:"nodesWithActions"`
	EdgesWithActions []EdgeWithActions `json:"edgesWithActions"`
}
