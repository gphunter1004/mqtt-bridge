package models

// Action Library Request
type ActionLibraryRequest struct {
	ActionType        string                           `json:"actionType" binding:"required"`
	ActionID          string                           `json:"actionId"`
	BlockingType      string                           `json:"blockingType"`
	ActionDescription string                           `json:"actionDescription"`
	Parameters        []ActionParameterTemplateRequest `json:"parameters"`
	Category          string                           `json:"category"`   // For organizing actions in library
	Tags              []string                         `json:"tags"`       // For searching and filtering
	IsReusable        bool                             `json:"isReusable"` // Whether this can be reused in multiple places
}

// Action Search Request
type ActionSearchRequest struct {
	SearchTerm   string   `json:"searchTerm"`
	ActionType   string   `json:"actionType"`
	BlockingType string   `json:"blockingType"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Limit        int      `json:"limit"`
	Offset       int      `json:"offset"`
}

// Action Clone Request
type ActionCloneRequest struct {
	SourceActionID uint   `json:"sourceActionId" binding:"required"`
	NewActionID    string `json:"newActionId" binding:"required"`
	NewDescription string `json:"newDescription"`
}

// Action Validation Request
type ActionValidationRequest struct {
	ActionType   string                           `json:"actionType" binding:"required"`
	Parameters   []ActionParameterTemplateRequest `json:"parameters"`
	SerialNumber string                           `json:"serialNumber"` // To validate against robot capabilities
}

// Action Export/Import
type ActionExportRequest struct {
	ActionIDs []uint `json:"actionIds" binding:"required"`
	Format    string `json:"format"` // "json", "yaml", "csv"
}

type ActionImportRequest struct {
	Actions []ActionTemplateRequest `json:"actions" binding:"required"`
	Options ActionImportOptions     `json:"options"`
}

type ActionImportOptions struct {
	OverwriteExisting bool `json:"overwriteExisting"`
	SkipDuplicates    bool `json:"skipDuplicates"`
	ValidateOnly      bool `json:"validateOnly"`
}

// Batch Operations
type ActionBatchRequest struct {
	ActionIDs []uint                 `json:"actionIds" binding:"required"`
	Operation string                 `json:"operation" binding:"required"` // "delete", "clone", "export"
	Options   map[string]interface{} `json:"options"`
}

// Batch Operations Response
type ActionBatchResponse struct {
	SuccessCount int                 `json:"successCount"`
	ErrorCount   int                 `json:"errorCount"`
	Results      []ActionBatchResult `json:"results"`
}

type ActionBatchResult struct {
	ActionID uint   `json:"actionId"`
	Status   string `json:"status"` // "success", "error"
	Message  string `json:"message"`
}

// Response models
type ActionLibraryResponse struct {
	Actions []ActionTemplateExtended `json:"actions"`
	Total   int64                    `json:"total"`
	Page    int                      `json:"page"`
	Size    int                      `json:"size"`
}

type ActionValidationResponse struct {
	IsValid       bool     `json:"isValid"`
	Errors        []string `json:"errors"`
	Warnings      []string `json:"warnings"`
	Suggestions   []string `json:"suggestions"`
	CanExecute    bool     `json:"canExecute"`
	MissingParams []string `json:"missingParams"`
}

type ActionImportResponse struct {
	ImportedCount int                  `json:"importedCount"`
	SkippedCount  int                  `json:"skippedCount"`
	ErrorCount    int                  `json:"errorCount"`
	Results       []ActionImportResult `json:"results"`
	Summary       ActionImportSummary  `json:"summary"`
}

type ActionImportResult struct {
	ActionType string `json:"actionType"`
	ActionID   string `json:"actionId"`
	Status     string `json:"status"` // "imported", "skipped", "error"
	Message    string `json:"message"`
	DatabaseID *uint  `json:"databaseId,omitempty"`
}

type ActionImportSummary struct {
	TotalActions     int      `json:"totalActions"`
	SuccessActions   int      `json:"successActions"`
	FailedActions    int      `json:"failedActions"`
	SkippedActions   int      `json:"skippedActions"`
	DuplicateActions int      `json:"duplicateActions"`
	NewActionTypes   []string `json:"newActionTypes"`
}

// Action Template with extended metadata
type ActionTemplateExtended struct {
	ActionTemplate
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	IsReusable   bool     `json:"isReusable"`
	UsageCount   int64    `json:"usageCount"`
	LastModified string   `json:"lastModified"`
	CreatedBy    string   `json:"createdBy"`
	Version      string   `json:"version"`
}
