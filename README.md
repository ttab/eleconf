# Eleconf

Tool for applying configuration to changes to the [Elephant repository](https://github.com/ttab/elephant-repository).

Enforces configuration for:

    * Schema versions
    * Meta type configuration
    * Document statuses
    * Document workflows

## Configuration

Eleconf uses HCL as its configuration language.

See example configuration files in the examples/tt folder.

### Schema sets

An organisations schemas are often split into several files for readability, but versioned together. Therefore schemas are configured as schema sets.

Schemas can either be loaded from a git repository, or over http(s).

Example schema set block:

``` hcl
schema_set "core" {
  version      = "v1.0.5-pre1"
  repository   = "https://github.com/ttab/revisorschemas.git"

  schemas = [
    "core",
    "core-planning",
    "core-metadoc",
  ]
}
```

As we specify a `repository` here the version must be a valid git tag. And the schemas files themselves are assumed to be located in the root of the repository.

If the schemas are to be loaded over http we can instead provide an URL template like so:

``` hcl
url_template = "https://raw.githubusercontent.com/ttab/revisorschemas/refs/tags/{{.Version}}/{{.Name}}.json"
```

### Document types

Document blocks are used to configure document types.

* `meta_doc` string: the document type that should be used for meta documents, optional.
* `statuses` string slice: all the statuses that are valid for the document type.
* `workflow` object: defintion of the workflow for the document, optional.
* `bounded_collection` bool: whether the document type is a bounded collection (finite and small number of documents).
* `time_expression` [object](https://pkg.go.dev/github.com/ttab/eleconf#TimeExpression): time expression used to extract timestamps.
* `label_expression` [object](https://pkg.go.dev/github.com/ttab/eleconf#LabelExpression): label expression used to extract labels.

#### Workflow object

* `step_zero` string: the workflow step that a document starts on, and reverts to after a new revision is created after a checkpoint status.
* `checkpoint` string: the name of the status to use as a checkpoint step, usually "usable" to signal that something is published.
* `negative_checkpoint` string: the checkpoint name to use when the checkpoint status is set with a negative version.
* `steps` string slice: the statuses that should be used as steps between checkpoints.

#### Example

``` hcl
document "core/article" {
  meta_doc = "core/article+meta"

  statuses = [
    "draft",
    "done",
    "approved",
    "withheld",
    "cancelled",
    "usable",
  ]

  workflow = {
    step_zero  = "draft"
    checkpoint = "usable"
    negative_checkpoint = "unpublished"
    steps      = [
      "draft",
      "done",
      "approved",
      "withheld",
      "cancelled",
    ]
  }
}
```

### Metrics

Metric blocks are used to configure metric kinds:

``` hcl
metric "charcount" {
  aggregation = "replace"
}
```

`aggregation` can be "replace" or "increment", defaults to "replace".

## Usage

All changes to schemas require lockfile update. So the first thing you have to do for a new configuration directory is to run the update command. This will not change anything in the repository, but will check that the referenced schema versions exist and update the lock file.

``` shellsession
eleconf update -dir examples/tt
```

To apply the configuration to a repository installation run `apply`: 

``` shellsession
eleconf apply -auth-env stage \
    -customer 000 \
    -endpoint https://repository.stage.tt.se \
    -dir examples/tt
```

This will compare the current configuration with the one declared in the configuration directory, detail the changes, and ask for confirmation before applying.

Example use:

``` shellsession
❯ go run ./cmd/eleconf apply -dir tt

~ schema downgrade tt v1.1.1 => v1.0.5-pre1
 Warning:  downgrading schema
+ status "print_done" for "tt/print-article"
- status "nonsense" for "tt/print-article"
~ update workflow for "core/event":
  &eleconf.DocumentWorkflow{
  	StepZero:           "draft",
  	Checkpoint:         "usable",
  	NegativeCheckpoint: "unpublished",
  	Steps: []string{
+ 		"draft",
  		"done",
  		"cancelled",
  	},
  }


Do you want to apply these changes? [y/n]: y

~ schema downgrade tt v1.1.1 => v1.0.5-pre1
+ status "print_done" for "tt/print-article"
- status "nonsense" for "tt/print-article"
~ update workflow for "core/event"

Configuration has been updated
```
