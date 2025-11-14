package eleconf

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/ttab/elephant-api/repository"
	"github.com/ttab/elephantine"
	"github.com/twitchtv/twirp"
)

func GetTypeConfigurationChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
) ([]ConfigurationChange, error) {
	schemas := clients.GetSchemas()

	wantMap := make(map[string]TypeConfigSpec)
	currMap := make(map[string]TypeConfigSpec)

	currTypes, err := schemas.GetDocumentTypes(ctx,
		&repository.GetDocumentTypesRequest{})
	if err != nil {
		return nil, fmt.Errorf("get current document types: %w", err)
	}

	for _, typ := range currTypes.Types {
		current, err := schemas.GetTypeConfiguration(ctx,
			&repository.GetTypeConfigurationRequest{
				Type: typ,
			})
		if err != nil && !elephantine.IsTwirpErrorCode(err, twirp.NotFound) {
			return nil, fmt.Errorf("get current type configuration: %w", err)
		}

		var spec TypeConfigSpec

		if current != nil && current.Configuration != nil {
			s := TypeConfigSpec{
				Bounded: current.Configuration.BoundedCollection,
			}

			for _, exp := range current.Configuration.TimeExpressions {
				s.TimeExpressions = append(s.TimeExpressions,
					TimeExpression{
						Expression: exp.Expression,
						Layout:     exp.Layout,
						Timezone:   exp.Timezone,
					})
			}

			for _, exp := range current.Configuration.LabelExpressions {
				s.LabelExpressions = append(s.LabelExpressions,
					LabelExpression{
						Expression: exp.Expression,
						Template:   exp.Template,
					})
			}

			spec = s
		}

		currMap[typ] = spec
	}

	for _, doc := range conf.Documents {
		wantMap[doc.Type] = TypeConfigSpec{
			Bounded:          doc.BoundedCollection,
			TimeExpressions:  doc.TimeExpressions,
			LabelExpressions: doc.LabelExpressions,
		}
	}

	var changes []ConfigurationChange

	for k := range wantMap {
		curr := currMap[k]
		want := wantMap[k]

		diff := cmp.Diff(curr, want)
		if diff == "" {
			continue
		}

		change := TypeConfigurationChange{
			diff:    diff,
			Type:    k,
			Current: curr,
			Wanted:  want,
		}

		changes = append(changes, &change)
	}

	for k := range currMap {
		curr := currMap[k]

		want, wanted := wantMap[k]
		if wanted {
			// Already captured by the wantMap iteration.
			continue
		}

		diff := cmp.Diff(curr, want)
		if diff == "" {
			continue
		}

		// We express all changes as updates to the type.
		change := TypeConfigurationChange{
			diff:    diff,
			Type:    k,
			Current: curr,
			Wanted:  want,
		}

		changes = append(changes, &change)
	}

	return changes, nil
}

type TypeConfigSpec struct {
	Bounded          bool
	TimeExpressions  []TimeExpression
	LabelExpressions []LabelExpression
}

var _ ConfigurationChange = &TypeConfigurationChange{}

type TypeConfigurationChange struct {
	diff string

	Operation ChangeOp
	Type      string
	Current   TypeConfigSpec
	Wanted    TypeConfigSpec
}

// Describe implements ConfigurationChange.
func (t *TypeConfigurationChange) Describe() (ChangeOp, string) {
	return OpUpdate, fmt.Sprintf(
		"update type configuration for %q:\n%s", t.Type, t.diff)
}

// Execute implements ConfigurationChange.
func (t *TypeConfigurationChange) Execute(ctx context.Context, c Clients) error {
	schemas := c.GetSchemas()

	config := repository.TypeConfiguration{
		BoundedCollection: t.Wanted.Bounded,
	}

	for _, exp := range t.Wanted.TimeExpressions {
		config.TimeExpressions = append(config.TimeExpressions,
			&repository.TypeTimeExpression{
				Expression: exp.Expression,
				Layout:     exp.Layout,
				Timezone:   exp.Timezone,
			})
	}

	for _, exp := range t.Wanted.LabelExpressions {
		config.LabelExpressions = append(config.LabelExpressions,
			&repository.LabelExpression{
				Expression: exp.Expression,
				Template:   exp.Template,
			})
	}

	_, err := schemas.ConfigureType(ctx, &repository.ConfigureTypeRequest{
		Type:          t.Type,
		Configuration: &config,
	})
	if err != nil {
		return fmt.Errorf("configure type %q: %w", t.Type, err)
	}

	return nil
}
