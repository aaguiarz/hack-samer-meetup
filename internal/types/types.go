package types

// EventMapping defines which Auth0 events map to which actions
type EventMapping struct {
	Type   string `yaml:"type" json:"type"`
	Action string `yaml:"action" json:"action"` // create, update, delete
}

// TupleDefinition defines the structure of an OpenFGA tuple
type TupleDefinition struct {
	User     string `yaml:"user" json:"user"`
	Relation string `yaml:"relation" json:"relation"`
	Object   string `yaml:"object" json:"object"`
}

// TupleMapping defines conditional mappings from Auth0 events to OpenFGA tuples
type TupleMapping struct {
	Condition string          `yaml:"condition" json:"condition"`
	Tuple     TupleDefinition `yaml:"tuple" json:"tuple"`
}

// MappingConfig contains the complete configuration for mapping Auth0 events
type MappingConfig struct {
	Events   []EventMapping `yaml:"events" json:"events"`
	Mappings []TupleMapping `yaml:"mappings" json:"mappings"`
}

// ProcessedTuple represents a tuple that has been processed with templates
type ProcessedTuple struct {
	User     string
	Relation string
	Object   string
}

// Auth0Event represents the structure of an Auth0 event
type Auth0Event struct {
	SpecVersion string                 `json:"specversion"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	ID          string                 `json:"id"`
	Time        string                 `json:"time"`
	Data        map[string]interface{} `json:"data"`
	A0Tenant    string                 `json:"a0tenant"`
	A0Stream    string                 `json:"a0stream"`
}
