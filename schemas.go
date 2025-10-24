package eleconf

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/ttab/elephant-api/repository"
	"github.com/ttab/elephantine"
	"github.com/ttab/revisor"
	"github.com/twitchtv/twirp"
)

func GetSchemaChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
	loaded []LoadedSchema,
) ([]ConfigurationChange, error) {
	schemas := clients.GetSchemas()

	active, err := schemas.ListActive(ctx,
		&repository.ListActiveSchemasRequest{})
	if err != nil {
		return nil, fmt.Errorf(
			"get active schemas: %w", err)
	}

	activateSchemas, err := getSchemaChanges(
		ctx, loaded, active.Schemas)
	if err != nil {
		return nil, err
	}

	err = checkDocsDefined(loaded, conf.Documents)
	if err != nil {
		return nil, err
	}

	updates := make([]ConfigurationChange,
		len(activateSchemas))

	for i := range activateSchemas {
		updates[i] = activateSchemas[i]
	}

	return updates, nil
}

var _ ConfigurationChange = schemaChange{}

func versionCompare(v1 string, v2 string) (int, error) {
	a, err := semver.NewVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v1, err)
	}

	b, err := semver.NewVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v2, err)
	}

	return a.Compare(b), nil
}

// Check that all doc types are defined in schemas.
func checkDocsDefined(
	schemas []LoadedSchema,
	docs []DocumentConfig,
) error {
	definedDocTypes := make(map[string]bool)

	for _, schema := range schemas {
		var cs revisor.ConstraintSet

		err := json.Unmarshal(schema.Data, &cs)
		if err != nil {
			return fmt.Errorf("invalid schema %s@%s",
				schema.Lock.Name, schema.Lock.Version)
		}

		for _, ds := range cs.Documents {
			if ds.Declares == "" {
				continue
			}

			definedDocTypes[ds.Declares] = true
		}
	}

	for _, dc := range docs {
		defined := definedDocTypes[dc.Type]
		if !defined {
			return fmt.Errorf(
				"document type %q has not been defined in schemas",
				dc.Type)
		}

		mDef := definedDocTypes[dc.MetaDocType]
		if dc.MetaDocType != "" && !mDef {
			return fmt.Errorf(
				"meta document type %q has not been defined in schemas",
				dc.Type)
		}
	}

	return nil
}

func getSchemaChanges(
	ctx context.Context,
	schemas []LoadedSchema,
	active []*repository.Schema,
) ([]schemaChange, error) {
	wantedLookup := make(map[string]LoadedSchema, len(schemas))

	for _, s := range schemas {
		wantedLookup[s.Lock.Name] = s
	}

	activeLookup := make(map[string]string, len(active))

	var changes []schemaChange

	for _, s := range active {
		activeLookup[s.Name] = s.Version

		wanted, ok := wantedLookup[s.Name]
		if !ok {
			changes = append(changes,
				schemaChange{
					Name:           s.Name,
					Deactivate:     true,
					CurrentVersion: s.Version,
				})

			continue
		}

		if s.Version != wanted.Lock.Version {
			cmp, err := versionCompare(
				s.Version,
				wanted.Lock.Version)
			if err != nil {
				return nil, fmt.Errorf(
					"compare %s versions: %w",
					s.Name, err)
			}

			// Up- or downgrade.
			changes = append(changes, schemaChange{
				Name:           s.Name,
				CurrentVersion: s.Version,
				Schema:         wanted,
				IsDowngrade:    cmp > 0,
			})

			continue
		}
	}

	for name, w := range wantedLookup {
		_, ok := activeLookup[name]
		if ok {
			continue
		}

		// New schemas.
		changes = append(changes, schemaChange{
			Name:   name,
			Schema: w,
		})
	}

	return changes, nil
}

type schemaChange struct {
	Name           string
	CurrentVersion string
	IsDowngrade    bool
	Deactivate     bool
	Schema         LoadedSchema
}

func (sc schemaChange) Warnings() []string {
	var warnings []string

	if sc.IsDowngrade {
		warnings = append(warnings,
			"downgrading schema")
	}

	return warnings
}

func (sc schemaChange) Execute(
	ctx context.Context,
	clients Clients,
) error {
	schemas := clients.GetSchemas()

	if sc.Deactivate {
		_, err := schemas.SetActive(ctx,
			&repository.SetActiveSchemaRequest{
				Name:       sc.Name,
				Deactivate: true,
			})

		return err
	}

	var activateExisting bool

	_, err := schemas.Register(ctx,
		&repository.RegisterSchemaRequest{
			Schema: &repository.Schema{
				Name:    sc.Name,
				Version: sc.Schema.Lock.Version,
				Spec:    string(sc.Schema.Data),
			},
			Activate: true,
		})
	if elephantine.IsTwirpErrorCode(err,
		twirp.FailedPrecondition) {
		// The schema verion already exists, do a simple activate
		// instead.
		activateExisting = true
	} else if err != nil {
		return fmt.Errorf("register schema: %w", err)
	}

	if !activateExisting {
		return nil
	}

	_, err = schemas.SetActive(ctx,
		&repository.SetActiveSchemaRequest{
			Name:    sc.Name,
			Version: sc.Schema.Lock.Version,
		})
	if err != nil {
		return fmt.Errorf("activate version: %w", err)
	}

	return nil
}

func (sc schemaChange) Describe() (ChangeOp, string) {
	switch {
	case sc.Deactivate:
		return OpRemove, fmt.Sprintf(
			"deactivate schema %s@%s",
			sc.Name, sc.CurrentVersion)

	case sc.CurrentVersion != "":
		op := "upgrade"
		if sc.IsDowngrade {
			op = "downgrade"
		}

		return OpUpdate, fmt.Sprintf(
			"schema %s %s %s => %s",
			op,
			sc.Name,
			sc.CurrentVersion,
			sc.Schema.Lock.Version,
		)

	default:
		return OpAdd, fmt.Sprintf(
			"activate schema %s@%s",
			sc.Name,
			sc.Schema.Lock.Version)

	}
}
