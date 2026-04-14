package eleconf

import (
	"context"
	"encoding/json"
	"fmt"

	rpcdoc "github.com/ttab/elephant-api/newsdoc"
	"github.com/ttab/elephant-api/repository"
	"github.com/ttab/revisor"
)

// GetSchemaChanges computes the schema changes needed to bring the remote
// state in line with the desired configuration. With schema generations,
// this produces at most one change: a generationChange that registers
// all schemas as a generation.
func GetSchemaChanges(
	ctx context.Context,
	clients Clients,
	conf *Config,
	loaded []LoadedSchema,
	exemplars []LoadedExemplar,
	activation repository.SchemaActivation,
) ([]ConfigurationChange, error) {
	schemas := clients.GetSchemas()

	active, err := schemas.ListActive(ctx,
		&repository.ListActiveSchemasRequest{})
	if err != nil {
		return nil, fmt.Errorf(
			"get active schemas: %w", err)
	}

	err = checkDocsDefined(loaded, conf.Documents)
	if err != nil {
		return nil, err
	}

	var currentExemplars []*repository.Exemplar

	if active.GenerationId > 0 && len(exemplars) > 0 {
		exRes, exErr := schemas.GetExemplars(ctx,
			&repository.GetExemplarsRequest{
				GenerationId: active.GenerationId,
			})
		if exErr != nil {
			return nil, fmt.Errorf(
				"get current exemplars: %w", exErr)
		}

		currentExemplars = exRes.Exemplars
	}

	// Check if the desired set matches the current active generation.
	if generationMatchesCurrent(loaded, exemplars, active, currentExemplars) {
		return nil, nil
	}

	return []ConfigurationChange{
		generationChange{
			Schemas:          loaded,
			Exemplars:        exemplars,
			Activation:       activation,
			Current:          active.Schemas,
			CurrentExemplars: currentExemplars,
		},
	}, nil
}

func generationMatchesCurrent(
	schemas []LoadedSchema,
	exemplars []LoadedExemplar,
	active *repository.ListActiveSchemasResponse,
	currentExemplars []*repository.Exemplar,
) bool {
	if len(schemas) != len(active.Schemas) {
		return false
	}

	activeMap := make(map[string]string, len(active.Schemas))
	for _, s := range active.Schemas {
		activeMap[s.Name] = s.Version
	}

	for _, s := range schemas {
		if activeMap[s.Lock.Name] != s.Lock.Version {
			return false
		}
	}

	if len(exemplars) != len(currentExemplars) {
		return false
	}

	exemplarMap := make(map[string]string, len(currentExemplars))
	for _, ex := range currentExemplars {
		exemplarMap[ex.Name] = ex.VersionHash
	}

	for _, ex := range exemplars {
		if exemplarMap[ex.Lock.Name] != ex.Lock.Hash {
			return false
		}
	}

	return true
}

var _ ConfigurationChange = generationChange{}

type generationChange struct {
	Schemas          []LoadedSchema
	Exemplars        []LoadedExemplar
	Activation       repository.SchemaActivation
	Current          []*repository.Schema
	CurrentExemplars []*repository.Exemplar
}

func (gc generationChange) Describe() (ChangeOp, string) {
	op := OpUpdate

	activationStr := "active"
	if gc.Activation == repository.SchemaActivation_ACTIVATION_PENDING {
		activationStr = "pending"
	}

	desc := fmt.Sprintf("register %s generation with %d schemas",
		activationStr, len(gc.Schemas))

	if len(gc.Exemplars) > 0 {
		desc += fmt.Sprintf(" and %d exemplars", len(gc.Exemplars))
	}

	currentMap := make(map[string]string, len(gc.Current))
	for _, s := range gc.Current {
		currentMap[s.Name] = s.Version
	}

	desiredMap := make(map[string]bool, len(gc.Schemas))

	for _, s := range gc.Schemas {
		desiredMap[s.Lock.Name] = true

		curVersion, exists := currentMap[s.Lock.Name]

		switch {
		case !exists:
			desc += fmt.Sprintf("\n  + %s@%s",
				s.Lock.Name, s.Lock.Version)
		case curVersion != s.Lock.Version:
			desc += fmt.Sprintf("\n  ~ %s@%s -> %s",
				s.Lock.Name, curVersion, s.Lock.Version)
		}
	}

	for _, s := range gc.Current {
		if !desiredMap[s.Name] {
			desc += fmt.Sprintf("\n  - %s@%s", s.Name, s.Version)
		}
	}

	curExemplarMap := make(map[string]string, len(gc.CurrentExemplars))
	for _, ex := range gc.CurrentExemplars {
		curExemplarMap[ex.Name] = ex.VersionHash
	}

	desiredExemplarMap := make(map[string]bool, len(gc.Exemplars))

	for _, ex := range gc.Exemplars {
		desiredExemplarMap[ex.Lock.Name] = true

		curHash, exists := curExemplarMap[ex.Lock.Name]

		switch {
		case !exists:
			desc += fmt.Sprintf("\n  + exemplar %s (%s)",
				ex.Lock.Name, ex.Lock.DocType)
		case curHash != ex.Lock.Hash:
			desc += fmt.Sprintf("\n  ~ exemplar %s (%s)",
				ex.Lock.Name, ex.Lock.DocType)
		}
	}

	for _, ex := range gc.CurrentExemplars {
		if !desiredExemplarMap[ex.Name] {
			desc += fmt.Sprintf("\n  - exemplar %s", ex.Name)
		}
	}

	return op, desc
}

func (gc generationChange) Execute(
	ctx context.Context,
	clients Clients,
) error {
	schemas := clients.GetSchemas()

	rpcSchemas := make([]*repository.Schema, 0, len(gc.Schemas))

	for _, s := range gc.Schemas {
		rpcSchemas = append(rpcSchemas, &repository.Schema{
			Name:    s.Lock.Name,
			Version: s.Lock.Version,
			Spec:    string(s.Data),
		})
	}

	rpcExemplars := make([]*rpcdoc.Document, 0, len(gc.Exemplars))

	for _, ex := range gc.Exemplars {
		rpcExemplars = append(rpcExemplars,
			rpcdoc.DocumentToRPC(ex.Document))
	}

	_, err := schemas.RegisterGeneration(ctx,
		&repository.RegisterGenerationRequest{
			Schemas:    rpcSchemas,
			Activation: gc.Activation,
			Exemplars:  rpcExemplars,
		})
	if err != nil {
		return fmt.Errorf("register generation: %w", err)
	}

	return nil
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
		baseType, _ := ParseDocumentType(dc.Type)

		if !definedDocTypes[baseType] {
			return fmt.Errorf(
				"document type %q has not been defined in schemas",
				dc.Type)
		}

		if dc.MetaDocType != "" && !definedDocTypes[dc.MetaDocType] {
			return fmt.Errorf(
				"meta document type %q has not been defined in schemas",
				dc.MetaDocType)
		}
	}

	return nil
}
