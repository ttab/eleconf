package eleconf

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/ttab/elephant-api/repository"
	"github.com/ttab/elephantine"
	"github.com/twitchtv/twirp"
)

func GetWorkflowChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
) ([]ConfigurationChange, error) {
	schemas := clients.GetSchemas()
	workflows := clients.GetWorkflows()

	wantMap := make(map[string]*DocumentWorkflow)
	currMap := make(map[string]*DocumentWorkflow)

	currTypes, err := schemas.GetDocumentTypes(ctx,
		&repository.GetDocumentTypesRequest{})
	if err != nil {
		return nil, fmt.Errorf("get current document types: %w", err)
	}

	for _, typ := range currTypes.Types {
		curr, err := workflows.GetWorkflow(ctx,
			&repository.GetWorkflowRequest{
				Type: typ,
			})
		if elephantine.IsTwirpErrorCode(err, twirp.NotFound) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf(
				"get current workflow for %q: %w",
				typ, err)
		}

		currMap[typ] = rpcToWorkflow(curr.Workflow)
	}

	for _, doc := range conf.Documents {
		if doc.Workflow == nil {
			continue
		}

		wantMap[doc.Type] = doc.Workflow
	}

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
	Current   *DocumentWorkflow
	Wanted    *DocumentWorkflow
}

// Describe implements ConfigurationChange.
func (d *DocWorkflowUpdate) Describe() (ChangeOp, string) {
	switch d.Operation {
	case OpAdd:
		diff := cmp.Diff(DocumentWorkflow{}, *d.Wanted)

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
) *DocumentWorkflow {
	c := DocumentWorkflow{
		StepZero:           r.StepZero,
		Checkpoint:         r.Checkpoint,
		NegativeCheckpoint: r.NegativeCheckpoint,
		Steps:              r.Steps,
	}

	return &c
}

func workflowToRPC(
	c *DocumentWorkflow,
) *repository.DocumentWorkflow {
	r := repository.DocumentWorkflow{
		StepZero:           c.StepZero,
		Checkpoint:         c.Checkpoint,
		NegativeCheckpoint: c.NegativeCheckpoint,
		Steps:              c.Steps,
	}

	return &r
}
