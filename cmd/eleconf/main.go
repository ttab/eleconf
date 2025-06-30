package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/joho/godotenv"
	"github.com/ttab/clitools"
	"github.com/ttab/eleconf"
	"github.com/ttab/elephant-api/repository"
	"github.com/ttab/elephantine"
	"github.com/ttab/revisor"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"
)

func main() {
	err := godotenv.Load()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Error("exiting: ",
			elephantine.LogKeyError, err)
		os.Exit(1)
	}

	initCmd := cli.Command{
		Name:        "init",
		Description: "Refresh schema lockfile",
		Action:      initAction,
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:  "dir",
				Usage: "Configuration directory",
				Value: cli.Path("."),
			},
		},
	}

	applyCmd := cli.Command{
		Name:        "apply",
		Description: "Applies elephant configuration",
		Action:      applyAction,
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:  "dir",
				Usage: "Configuration directory",
				Value: cli.Path("."),
			},
		},
	}

	app := cli.App{
		Name:  "eleconf",
		Usage: "Elephant repository configuration tool",
		Commands: []*cli.Command{
			&initCmd,
			&applyCmd,
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "auth-env",
				Usage:   "Auth environment",
				EnvVars: []string{"AUTH_ENV"},
			},
			&cli.StringFlag{
				Name:     "endpoint",
				Usage:    "Elephant repository endpoint",
				EnvVars:  []string{"ENDPOINT"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "customer",
				Usage:    "Elephant customer",
				EnvVars:  []string{"CUSTOMER"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "client-id",
				Usage:   "Client ID",
				EnvVars: []string{"CLIENT_ID"},
			},
			&cli.StringFlag{
				Name:    "client-secret",
				Usage:   "Client secret",
				EnvVars: []string{"CLIENT_SECRET"},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		println("error: ", err.Error())
		os.Exit(1)
	}
}

func initAction(c *cli.Context) error {
	dir := c.Path("dir")

	ctx := c.Context

	conf, err := eleconf.ReadConfigFromDirectory(dir)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}

	lock, err := eleconf.LoadLockFile(eleconf.LockFilePath(dir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load lock file: %w", err)
	}

	var schemas []eleconf.LoadedSchema

	for _, set := range conf.SchemaSets {
		loaded, err := eleconf.LoadSchemaSet(ctx, set, lock, true)
		if err != nil {
			return fmt.Errorf("load schema set %q: %w", set.Name, err)
		}

		schemas = append(schemas, loaded...)
	}

	lock = eleconf.NewSchemaLockFile(schemas)

	err = lock.Save(eleconf.LockFilePath(dir))
	if err != nil {
		return fmt.Errorf("save lock file: %w", err)
	}

	return nil
}

func applyAction(c *cli.Context) error {
	dir := c.Path("dir")

	ctx := c.Context

	conf, err := eleconf.ReadConfigFromDirectory(dir)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}

	lock, err := eleconf.LoadLockFile(eleconf.LockFilePath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("missing lock file, run eleconf init")
	} else if err != nil {
		return fmt.Errorf("load lock file: %w", err)
	}

	clients, err := getClients(ctx, c)
	if err != nil {
		return fmt.Errorf("get API clients: %w", err)
	}

	active, err := clients.Schemas.ListActive(ctx,
		&repository.ListActiveSchemasRequest{})
	if err != nil {
		return fmt.Errorf("get active schemas: %w", err)
	}

	schemas, activateSchemas, err := getSchemaChanges(
		ctx, clients,
		conf.SchemaSets, lock, active.Schemas)
	if err != nil {
		return err
	}

	err = checkDocsDefined(schemas, conf.Documents)
	if err != nil {
		return err
	}

	// TODO: Check metadoc type registration.
	// TODO: Check if statuses exist.
	// TODO: Check workflows.

	for _, ac := range activateSchemas {
		if ac.CurrentVersion != "" {
			cmp, err := versionCompare(ac.CurrentVersion, ac.Schema.Lock.Version)
			if err != nil {
				return fmt.Errorf("compare %s versions: %w",
					ac.Schema.Lock.Name, err)
			}

			if cmp == 0 {
				continue
			}

			action := "Upgrade"
			if cmp > 0 {
				action = "Downgrade"
			}

			println(action, ac.Schema.Lock.Name, ac.CurrentVersion,
				"=>", ac.Schema.Lock.Version)
		} else {
			println("Add", ac.Schema.Lock.Name, ac.Schema.Lock.Version)
		}
	}

	return nil
}

const (
	metaOpRegister      = 1
	metaOpRegisterUse   = 2
	metaOpUnregisterUse = 3
)

type metaTypeChange struct{}

func getMetaTypeChanges(
	ctx context.Context,
	clients *clients,
	conf *eleconf.Config,
) ([]metaTypeChange, error) {
	metaTypes, err := clients.Schemas.GetMetaTypes(
		ctx, &repository.GetMetaTypesRequest{})
	if err != nil {
		return nil, fmt.Errorf("get current meta types: %w", err)
	}

	definedLookup := make(map[string]bool)
	currentLookup := make(map[string]string)

	for _, m := range metaTypes.Types {
		definedLookup[m.Name] = true

		for _, main := range m.UsedBy {
			currentLookup[main] = m.Name
		}
	}

	usedLookup := make(map[string]string)

	for _, doc := range conf.Documents {
		if doc.MetaDocType == "" {
			continue
		}

		usedLookup[doc.Type] = doc.MetaDocType
	}
}

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
func checkDocsDefined(schemas []eleconf.LoadedSchema, docs []eleconf.DocumentConfig) error {
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
	clients *clients,
	sets []eleconf.SchemaSet,
	lock *eleconf.SchemaLockfile,
	active []*repository.Schema,
) ([]eleconf.LoadedSchema, []schemaChange, error) {
	var schemas []eleconf.LoadedSchema

	for _, set := range sets {
		loaded, err := eleconf.LoadSchemaSet(ctx, set, lock, false)
		if err != nil {
			return nil, nil, fmt.Errorf("load schema set %q: %w", set.Name, err)
		}

		schemas = append(schemas, loaded...)
	}

	wantedLookup := make(map[string]eleconf.LoadedSchema, len(schemas))

	for _, s := range schemas {
		wantedLookup[s.Lock.Name] = s
	}

	activeLookup := make(map[string]string, len(active))

	var (
		deactivateSchemas []string
		changes           []schemaChange
	)

	for _, s := range active {
		activeLookup[s.Name] = s.Version

		wanted, ok := wantedLookup[s.Name]
		if !ok {
			deactivateSchemas = append(deactivateSchemas, s.Name)

			continue
		}

		if s.Version != wanted.Lock.Version {
			// Up- or downgrade.
			changes = append(changes, schemaChange{
				CurrentVersion: s.Version,
				Schema:         wanted,
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
			Schema: w,
		})
	}

	return schemas, changes, nil
}

type schemaChange struct {
	CurrentVersion string
	Deactivate     bool
	Schema         eleconf.LoadedSchema
}

type clients struct {
	Workflows repository.Workflows
	Schemas   repository.Schemas
}

func getClients(
	ctx context.Context,
	c *cli.Context,
) (*clients, error) {
	endpoint := c.String("endpoint")
	clientID := c.String("client-id")
	clientSecret := c.String("client-secret")
	env := c.String("auth-env")

	if clientID == "" {
		clientID = clitools.DefaultApplicationID
	}

	var oidcServer string

	switch env {
	case "stage":
		oidcServer = clitools.StageOIDCServer
	case "prod":
		oidcServer = clitools.ProdOIDCServer
	default:
		return nil, fmt.Errorf("unknown environment %q", env)
	}

	oidcURL, err := clitools.OIDCConfigURL(oidcServer, "elephant")
	if err != nil {
		return nil, fmt.Errorf("create OIDC config URL: %w", err)
	}

	conf, err := clitools.NewConfigurationHandler[struct{}](
		"eleconf", clientID, env, oidcURL,
	)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}

	var token oauth2.TokenSource

	scopes := []string{
		"workflow_admin", "schema_admin",
	}

	if clientSecret != "" {
		t, err := conf.GetClientAccessToken(
			ctx, env, clientID, clientSecret, scopes)
		if err != nil {
			return nil, fmt.Errorf(
				"get client access token: %w", err)
		}

		token = t
	} else {
		t, err := conf.GetAccessToken(ctx, env, scopes)
		if err != nil {
			return nil, fmt.Errorf("get access token: %w", err)
		}

		token = oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: t.Token,
		})
	}

	err = conf.Save()
	if err != nil {
		slog.Warn("failed to save configuration", "err", err)
	}

	client := oauth2.NewClient(ctx, token)

	return &clients{
		Workflows: repository.NewWorkflowsProtobufClient(endpoint, client),
		Schemas:   repository.NewSchemasProtobufClient(endpoint, client),
	}, nil
}
