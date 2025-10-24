package eleconf

import (
	"context"
	"fmt"

	"github.com/ttab/elephant-api/repository"
)

func GetMetricsChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
) ([]ConfigurationChange, error) {
	metrics := clients.GetMetrics()

	wantMap := make(map[string]MetricAggregation)
	currMap := make(map[string]MetricAggregation)

	current, err := metrics.GetKinds(ctx, &repository.GetMetricKindsRequest{})
	if err != nil {
		return nil, fmt.Errorf("get current kinds: %w", err)
	}

	for _, m := range current.Kinds {
		var agg MetricAggregation

		switch m.Aggregation {
		case repository.MetricAggregation_INCREMENT:
			agg = MetricAggregationIncrement
		case repository.MetricAggregation_REPLACE:
			agg = MetricAggregationReplace
		default:
			return nil, fmt.Errorf(
				"unexpected repository.MetricAggregation: %#v", m.Aggregation)
		}

		currMap[m.Name] = agg
	}

	for _, m := range conf.Metric {
		switch m.Aggregation {
		case MetricAggregationIncrement:
		case MetricAggregationReplace:
		case "":
			m.Aggregation = MetricAggregationReplace
		default:
			return nil, fmt.Errorf(
				"unexpected MetricAggregation: %#v", m.Aggregation)
		}

		wantMap[m.Kind] = m.Aggregation
	}

	var changes []ConfigurationChange

	for k, currAgg := range currMap {
		agg, wanted := wantMap[k]
		if !wanted {
			changes = append(changes, &MetricUpdate{
				Operation: OpRemove,
				Kind:      k,
			})

			continue
		}

		if currAgg == agg {
			continue
		}

		changes = append(changes, &MetricUpdate{
			Operation:      OpUpdate,
			Kind:           k,
			OldAggregation: currAgg,
			Aggregation:    agg,
		})
	}

	for k, agg := range wantMap {
		_, exists := currMap[k]
		if exists {
			continue
		}

		changes = append(changes, &MetricUpdate{
			Operation:   OpAdd,
			Kind:        k,
			Aggregation: agg,
		})
	}

	return changes, nil
}

var _ ConfigurationChange = &MetricUpdate{}

type MetricUpdate struct {
	Operation      ChangeOp
	Kind           string
	OldAggregation MetricAggregation
	Aggregation    MetricAggregation
}

// Describe implements ConfigurationChange.
func (m *MetricUpdate) Describe() (ChangeOp, string) {
	var desc string

	switch m.Operation {
	case OpAdd:
		desc = fmt.Sprintf("add metric kind %q (aggregation %q)",
			m.Kind, m.Aggregation)
	case OpRemove:
		desc = fmt.Sprintf("remove metric kind %q",
			m.Kind)
	case OpUpdate:
		desc = fmt.Sprintf("update metric kind %q (aggregation %q => %q)",
			m.Kind, m.OldAggregation, m.Aggregation)
	default:
		panic(fmt.Sprintf("unexpected internal.ChangeOp: %#v", m.Operation))
	}

	return m.Operation, desc
}

// Execute implements ConfigurationChange.
func (m *MetricUpdate) Execute(ctx context.Context, c Clients) error {
	metrics := c.GetMetrics()

	switch m.Operation {
	case OpAdd, OpUpdate:
		agg := repository.MetricAggregation_REPLACE
		if m.Aggregation == MetricAggregationIncrement {
			agg = repository.MetricAggregation_INCREMENT
		}

		_, err := metrics.RegisterKind(ctx,
			&repository.RegisterMetricKindRequest{
				Name:        m.Kind,
				Aggregation: agg,
			})
		if err != nil {
			return fmt.Errorf("update metric kind: %w", err)
		}
	case OpRemove:

		_, err := metrics.DeleteKind(ctx,
			&repository.DeleteMetricKindRequest{
				Name: m.Kind,
			})
		if err != nil {
			return fmt.Errorf("delete metric kind: %w", err)
		}
	default:
		panic(fmt.Sprintf("unexpected internal.ChangeOp: %#v", m.Operation))
	}

	return nil
}
