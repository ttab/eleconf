package eleconf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

type Config struct {
	Documents  []DocumentConfig `hcl:"document,block"`
	SchemaSets []SchemaSet      `hcl:"schema_set,block"`
	Metric     []MetricKind     `hcl:"metric,block"`
}

type DocumentConfig struct {
	Type              string             `hcl:"type,label"`
	MetaDocType       string             `hcl:"meta_doc,optional"`
	Statuses          []string           `hcl:"statuses,optional"`
	Workflow          *DocumentWorkflow  `hcl:"workflow,optional"`
	Attachments       []AttachmentConfig `hcl:"attachment,block"`
	BoundedCollection bool               `hcl:"bounded_collection,optional"`
	TimeExpressions   []TimeExpression   `hcl:"time_expression,block"`
	LabelExpressions  []LabelExpression  `hcl:"label_expression,block"`
}

type TimeExpression struct {
	Expression string `hcl:"expression"`
	Layout     string `hcl:"layout,optional"`
	Timezone   string `hcl:"timezone,optional"`
}

type LabelExpression struct {
	Expression string `hcl:"expression"`
	Template   string `hcl:"template"`
}

type DocumentWorkflow struct {
	StepZero           string   `cty:"step_zero"`
	Checkpoint         string   `cty:"checkpoint"`
	NegativeCheckpoint string   `cty:"negative_checkpoint"`
	Steps              []string `cty:"steps"`
}

type SchemaSet struct {
	Name        string   `hcl:"name,label"`
	Version     string   `hcl:"version"`
	URLTemplate string   `hcl:"url_template,optional"`
	Repository  string   `hcl:"repository,optional"`
	Schemas     []string `hcl:"schemas"`
}

type AttachmentConfig struct {
	Name          string   `hcl:"name,label"`
	Required      bool     `hcl:"required"`
	MatchMimetype []string `hcl:"match_mimetype"`
}

type MetricKind struct {
	Kind        string            `hcl:"kind,label"`
	Aggregation MetricAggregation `hcl:"aggregation,optional"`
}

type MetricAggregation string

const (
	MetricAggregationReplace   MetricAggregation = "replace"
	MetricAggregationIncrement MetricAggregation = "increment"
)

type SchemaLock struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

func LockFilePath(dir string) string {
	return filepath.Join(dir, "schema.lock.json")
}

func ReadConfigFromDirectory(path string) (*Config, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("list directory contents: %w", err)
	}

	var tutti Config

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".hcl") {
			continue
		}

		c, err := parseFile(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf(
				"parse %q: %w", entry.Name(), err)
		}

		tutti.SchemaSets = append(tutti.SchemaSets, c.SchemaSets...)
		tutti.Documents = append(tutti.Documents, c.Documents...)
		tutti.Metric = append(tutti.Metric, c.Metric...)
	}

	return &tutti, nil
}

func parseFile(path string) (*Config, error) {
	var c Config

	err := hclsimple.DecodeFile(path, nil, &c)
	if err != nil {
		return nil, fmt.Errorf(
			"decode file: %w", err)
	}

	for _, m := range c.Metric {
		switch m.Aggregation {
		case "", MetricAggregationIncrement, MetricAggregationReplace:
		default:
			return nil, fmt.Errorf(
				"unknown %q metric aggregation %q",
				m.Kind,
				m.Aggregation)
		}
	}

	return &c, nil
}

type LoadedSchema struct {
	Lock SchemaLock
	Data []byte
}

func LoadSchemaSet(
	ctx context.Context,
	set SchemaSet,
	lockfile *SchemaLockfile,
	init bool,
) (_ []LoadedSchema, outErr error) {
	if lockfile == nil && !init {
		return nil, errors.New("missing lock file, run eleconf update")
	}

	var source SchemaSource

	switch {
	case strings.HasPrefix(set.URLTemplate, "https://"):
		s, err := NewHttpSchemaSource(
			ctx, set.URLTemplate, set.Version)
		if err != nil {
			return nil, fmt.Errorf("create HTTP source: %w", err)
		}

		source = s
	case set.Repository != "":
		s, err := NewGitSchemaSource(ctx, set.Repository, set.Version)
		if err != nil {
			return nil, fmt.Errorf("create git source: %w", err)
		}

		source = s
	default:
		return nil, errors.New("unknown schema source")
	}

	var list []LoadedSchema

	for _, name := range set.Schemas {
		schema, err := source.LoadSchema(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("load schema %q: %w", name, err)
		}

		if lockfile != nil {
			err := lockfile.Check(name, schema, init)
			if err != nil {
				return nil, err
			}
		}

		list = append(list, schema)
	}

	return list, nil
}
