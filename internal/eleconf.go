package internal

import (
	"context"
	"fmt"

	"github.com/ttab/eleconf"
	"github.com/ttab/elephant-api/repository"
)

type Clients interface {
	GetWorkflows() repository.Workflows
	GetSchemas() repository.Schemas
	GetMetrics() repository.Metrics
}

func GetChanges(
	ctx context.Context,
	clients Clients,
	conf *eleconf.Config,
	schemas []eleconf.LoadedSchema,
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
