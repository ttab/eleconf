package internal

import "github.com/ttab/elephant-api/repository"

type Clients interface {
	GetWorkflows() repository.Workflows
	GetSchemas() repository.Schemas
	GetMetrics() repository.Metrics
}
