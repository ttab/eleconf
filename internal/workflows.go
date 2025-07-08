package internal

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/ttab/eleconf"
	"github.com/ttab/elephant-api/repository"
	"github.com/ttab/elephantine"
	"github.com/twitchtv/twirp"
)

func GetWorkflowChanges(
	ctx context.Context,
	clients Clients,
	conf *eleconf.Config,
) ([]ConfigurationChange, error) {
	workflows := clients.GetWorkflows()

	wantMap := make(map[string]*eleconf.DocumentWorkflow)
	currMap := make(map[string]*eleconf.DocumentWorkflow)

	for _, doc := range conf.Documents {
		curr, err := workflows.GetWorkflow(ctx,
			&repository.GetWorkflowRequest{
				Type: doc.Type,
			})
		if err != nil &&
			!elephantine.IsTwirpErrorCode(err, twirp.NotFound) {
			return nil, fmt.Errorf(
				"get current workflow for %q: %w",
				doc.Type, err)
		}

		if curr != nil {
			currMap[doc.Type] = rpcToWorkflow(curr.Workflow)
		}

		if doc.Workflow != nil {
			wantMap[doc.Type] = doc.Workflow
		}
	}

	// TODO: No way to enumerate all workflows or document types in the
	// repo. Removing a doc type from config will therefore not remove the
	// workflow.

	var changes []ConfigurationChange

	for k := range wantMap {
		curr, ok := currMap[k]
		if !ok {
			changes = append(changes, &DocWorkflowUpdate{
				Type:      k,
				Operation: OpAdd,
				Wanted:    wantMap[k],
			})

			continue
		}

		up := DocWorkflowUpdate{
			Type:      k,
			Operation: OpUpdate,
			Current:   curr,
			Wanted:    wantMap[k],
		}

		diff := cmp.Diff(up.Current, up.Wanted)
		if diff == "" {
			continue
		}

		up.diff = diff

		changes = append(changes, &up)
	}

	for k := range currMap {
		_, wanted := wantMap[k]
		if !wanted {
			changes = append(changes, &DocWorkflowUpdate{
				Type:      k,
				Operation: OpRemove,
				Current:   currMap[k],
			})
		}
	}

	return changes, nil
}

var _ ConfigurationChange = &DocWorkflowUpdate{}

type DocWorkflowUpdate struct {
	diff string

	Operation ChangeOp
	Type      string
	Current   *eleconf.DocumentWorkflow
	Wanted    *eleconf.DocumentWorkflow
}

// Describe implements ConfigurationChange.
func (d *DocWorkflowUpdate) Describe() (ChangeOp, string) {
	switch d.Operation {
	case OpAdd:
		diff := cmp.Diff(eleconf.DocumentWorkflow{}, *d.Wanted)

		return OpAdd, fmt.Sprintf(
			"add workflow for %q:\n%s", d.Type, diff)
	case OpRemove:
		return OpRemove, fmt.Sprintf(
			"remove workflow for %q", d.Type)
	case OpUpdate:
		return OpUpdate, fmt.Sprintf(
			"update workflow for %q:\n%s", d.Type, d.diff)
	default:
		panic(fmt.Sprintf("unexpected internal.ChangeOp: %#v", d.Operation))
	}
}

// Execute implements ConfigurationChange.
func (d *DocWorkflowUpdate) Execute(ctx context.Context, c Clients) error {
	wf := c.GetWorkflows()

	switch d.Operation {
	case OpAdd, OpUpdate:
		_, err := wf.SetWorkflow(ctx, &repository.SetWorkflowRequest{
			Type:     d.Type,
			Workflow: workflowToRPC(d.Wanted),
		})
		if err != nil {
			return fmt.Errorf("set workflow: %w", err)
		}

		return nil
	case OpRemove:
		_, err := wf.DeleteWorkflow(ctx,
			&repository.DeleteWorkflowRequest{
				Type: d.Type,
			})
		if err != nil {
			return fmt.Errorf("delete workflow: %w", err)
		}

		return nil
	default:
		panic(fmt.Sprintf("unexpected internal.ChangeOp: %#v", d.Operation))
	}
}

func rpcToWorkflow(
	r *repository.DocumentWorkflow,
) *eleconf.DocumentWorkflow {
	c := eleconf.DocumentWorkflow{
		StepZero:           r.StepZero,
		Checkpoint:         r.Checkpoint,
		NegativeCheckpoint: r.NegativeCheckpoint,
		Steps:              r.Steps,
	}

	return &c
}

func workflowToRPC(
	c *eleconf.DocumentWorkflow,
) *repository.DocumentWorkflow {
	r := repository.DocumentWorkflow{
		StepZero:           c.StepZero,
		Checkpoint:         c.Checkpoint,
		NegativeCheckpoint: c.NegativeCheckpoint,
		Steps:              c.Steps,
	}

	return &r
}
