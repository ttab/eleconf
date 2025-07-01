package internal

import (
	"context"
	"fmt"

	"github.com/ttab/eleconf"
	"github.com/ttab/elephant-api/repository"
)

func GetStatusChanges(
	ctx context.Context,
	clients Clients,
	conf *eleconf.Config,
) ([]ConfigurationChange, error) {
	workflows := clients.GetWorkflows()

	var changes []ConfigurationChange

	for _, doc := range conf.Documents {
		current, err := workflows.GetStatuses(ctx,
			&repository.GetStatusesRequest{
				Type: doc.Type,
			})
		if err != nil {
			return nil, fmt.Errorf(
				"get statuses for %q: %w",
				doc.Type, err)
		}

		currMap := make(map[string]bool)
		wantMap := make(map[string]bool)

		for _, stat := range current.Statuses {
			currMap[stat.Name] = true
		}

		for _, stat := range doc.Statuses {
			wantMap[stat] = true

			if !currMap[stat] {
				changes = append(changes, statusChange{
					Type:   doc.Type,
					Status: stat,
				})
			}
		}

		for stat := range currMap {
			if !wantMap[stat] {
				changes = append(changes, statusChange{
					Type:    doc.Type,
					Status:  stat,
					Disable: true,
				})
			}
		}
	}

	return changes, nil
}

var _ ConfigurationChange = statusChange{}

type statusChange struct {
	Type    string
	Status  string
	Disable bool
}

// Describe implements ConfigurationChange.
func (s statusChange) Describe() (ChangeOp, string) {
	if s.Disable {
		return OpRemove, fmt.Sprintf(
			"status %q for %q", s.Status, s.Type)
	}

	return OpAdd, fmt.Sprintf(
		"status %q for %q", s.Status, s.Type)
}

// Execute implements ConfigurationChange.
func (s statusChange) Execute(ctx context.Context, c Clients) error {
	workflows := c.GetWorkflows()

	_, err := workflows.UpdateStatus(ctx,
		&repository.UpdateStatusRequest{
			Type:     s.Type,
			Name:     s.Status,
			Disabled: s.Disable,
		})
	if err != nil {
		return fmt.Errorf("update status in repository: %w", err)
	}

	return nil
}
