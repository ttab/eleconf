package internal

import (
	"context"
	"errors"
	"fmt"

	"github.com/ttab/eleconf"
	"github.com/ttab/elephant-api/repository"
)

func GetMetaTypeChanges(
	ctx context.Context,
	clients Clients,
	conf *eleconf.Config,
) ([]ConfigurationChange, error) {
	schemas := clients.GetSchemas()

	metaTypes, err := schemas.GetMetaTypes(
		ctx, &repository.GetMetaTypesRequest{})
	if err != nil {
		return nil, fmt.Errorf("get current meta types: %w", err)
	}

	// Lookup maps for the current configured meta types.
	definedLookup := make(map[string]bool)
	currentLookup := make(map[string]string)

	for _, m := range metaTypes.Types {
		definedLookup[m.Name] = true

		for _, main := range m.UsedBy {
			currentLookup[main] = m.Name
		}
	}

	// Main to meta type use in the document configuration.
	usedLookup := make(map[string]string)
	metaUsed := make(map[string]bool)

	for _, doc := range conf.Documents {
		if doc.MetaDocType == "" {
			continue
		}

		usedLookup[doc.Type] = doc.MetaDocType
		metaUsed[doc.MetaDocType] = true
	}

	var changes []ConfigurationChange

	registerRequested := map[string]bool{}
	unregisterRequested := map[string]bool{}

	for mainType, defMeta := range usedLookup {
		if !definedLookup[defMeta] && !registerRequested[defMeta] {
			changes = append(changes, metaTypeChange{
				Change:   metaOpRegister,
				MetaType: defMeta,
			})

			registerRequested[defMeta] = true
		}

		currMeta, ok := currentLookup[mainType]

		switch {
		case !ok:
			changes = append(changes, metaTypeChange{
				Change:   metaOpRegisterUse,
				MainType: mainType,
				MetaType: defMeta,
			})
		case currMeta != defMeta:
			changes = append(changes, metaTypeChange{
				Change:   metaOpUnregisterUse,
				MainType: mainType,
				MetaType: currMeta,
			})

			changes = append(changes, metaTypeChange{
				Change:   metaOpRegisterUse,
				MainType: mainType,
				MetaType: defMeta,
			})
		}
	}

	for mainType, currMeta := range currentLookup {
		_, ok := usedLookup[mainType]
		if ok {
			continue
		}

		changes = append(changes, metaTypeChange{
			Change:   metaOpUnregisterUse,
			MainType: mainType,
			MetaType: currMeta,
		})
	}

	for metaType := range definedLookup {
		if metaUsed[metaType] || unregisterRequested[metaType] {
			continue
		}

		changes = append(changes, metaTypeChange{
			Change: metaOpUnregister,
		})

		unregisterRequested[metaType] = true
	}

	return changes, nil
}

type metaOp int

const (
	metaOpRegister      metaOp = 1
	metaOpUnregister    metaOp = 2
	metaOpRegisterUse   metaOp = 3
	metaOpUnregisterUse metaOp = 4
)

var _ ConfigurationChange = metaTypeChange{}

type metaTypeChange struct {
	Change   metaOp
	MainType string
	MetaType string
}

// Execute implements ConfigurationChange.
func (mc metaTypeChange) Execute(
	ctx context.Context,
	c Clients,
) error {
	schemas := c.GetSchemas()

	switch mc.Change {
	case metaOpRegister:
		_, err := schemas.RegisterMetaType(ctx,
			&repository.RegisterMetaTypeRequest{
				Type: mc.MetaType,
			})

		return err
	case metaOpRegisterUse:
		_, err := schemas.RegisterMetaTypeUse(ctx,
			&repository.RegisterMetaTypeUseRequest{
				MainType: mc.MainType,
				MetaType: mc.MetaType,
			})

		return err
	case metaOpUnregister:
		return errors.New("not possible yet to unregister meta types")
	case metaOpUnregisterUse:
		return errors.New("not possible yet to unregister meta type use")
	default:
		panic(fmt.Sprintf("unexpected main.metaOp: %#v", mc.Change))
	}
}

func (mc metaTypeChange) Describe() (ChangeOp, string) {
	switch mc.Change {
	case metaOpRegister:
		return OpAdd, fmt.Sprintf("meta type %q", mc.MetaType)
	case metaOpRegisterUse:
		return OpAdd, fmt.Sprintf("meta type %q for %q", mc.MetaType, mc.MainType)
	case metaOpUnregister:
		return OpRemove, fmt.Sprintf("meta type %q", mc.MetaType)
	case metaOpUnregisterUse:
		return OpRemove, fmt.Sprintf("meta type %q for %q", mc.MetaType, mc.MainType)
	default:
		panic(fmt.Sprintf("unexpected main.metaOp: %#v", mc.Change))
	}
}
