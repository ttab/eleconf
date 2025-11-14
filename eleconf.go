package eleconf

import (
	"context"
	"fmt"

	"github.com/ttab/elephant-api/repository"
)

type Clients interface {
	GetWorkflows() repository.Workflows
	GetSchemas() repository.Schemas
	GetMetrics() repository.Metrics
}

var _ Clients = &StaticClients{}

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

func GetChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
	schemas []LoadedSchema,
) ([]ConfigurationChange, error) {
	var changes []ConfigurationChange

	scChanges, err := GetSchemaChanges(
		ctx, clients, conf, schemas)
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
