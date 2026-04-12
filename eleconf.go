package eleconf

import (
	"context"
	"fmt"

	"github.com/ttab/elephant-api/repository"
)

// Clients provides access to the elephant repository API services.
type Clients interface {
	GetWorkflows() repository.Workflows
	GetSchemas() repository.Schemas
	GetMetrics() repository.Metrics
}

var _ Clients = &StaticClients{}

// StaticClients is a simple Clients implementation with fixed service clients.
type StaticClients struct {
	Workflows repository.Workflows
	Schemas   repository.Schemas
	Metrics   repository.Metrics
}

// GetMetrics implements Clients.
func (c *StaticClients) GetMetrics() repository.Metrics {
	return c.Metrics
}

// GetSchemas implements Clients.
func (c *StaticClients) GetSchemas() repository.Schemas {
	return c.Schemas
}

// GetWorkflows implements Clients.
func (c *StaticClients) GetWorkflows() repository.Workflows {
	return c.Workflows
}

// GetChanges computes all configuration changes needed to bring the remote
// state in line with the desired configuration. Schema changes use
// RegisterGeneration with the given activation status.
func GetChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
	schemas []LoadedSchema,
	exemplars []LoadedExemplar,
	activation repository.SchemaActivation,
) ([]ConfigurationChange, error) {
	var changes []ConfigurationChange

	scChanges, err := GetSchemaChanges(
		ctx, clients, conf, schemas, exemplars, activation)
	if err != nil {
		return nil, fmt.Errorf("calculate schema changes: %w", err)
	}

	changes = append(changes, scChanges...)

	mtChanges, err := GetMetaTypeChanges(ctx, clients, conf)
	if err != nil {
		return nil, fmt.Errorf("calculate meta type changes: %w", err)
	}

	changes = append(changes, mtChanges...)

	stChanges, err := GetStatusChanges(ctx, clients, conf)
	if err != nil {
		return nil, fmt.Errorf("calculate status changes: %w", err)
	}

	changes = append(changes, stChanges...)

	wfChanges, err := GetWorkflowChanges(ctx, clients, conf)
	if err != nil {
		return nil, fmt.Errorf("calculate workflow changes: %w", err)
	}

	changes = append(changes, wfChanges...)

	meChanges, err := GetMetricsChanges(ctx, clients, conf)
	if err != nil {
		return nil, fmt.Errorf("calculate metrics changes: %w", err)
	}

	changes = append(changes, meChanges...)

	typChanges, err := GetTypeConfigurationChanges(ctx, clients, conf)
	if err != nil {
		return nil, fmt.Errorf("calculate type changes: %w", err)
	}

	changes = append(changes, typChanges...)

	return changes, nil
}

// GetGenerationChanges computes only the schema generation change, skipping
// all other configuration domains. Used by the "generation pending" command.
func GetGenerationChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
	schemas []LoadedSchema,
	exemplars []LoadedExemplar,
	activation repository.SchemaActivation,
) ([]ConfigurationChange, error) {
	return GetSchemaChanges(
		ctx, clients, conf, schemas, exemplars, activation)
}
