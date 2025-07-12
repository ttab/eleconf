package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/ttab/eleconf"
	"github.com/ttab/eleconf/internal"
	"github.com/ttab/elephantine"
	"github.com/urfave/cli/v2"
)

func main() {
	err := godotenv.Load()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Error("exiting: ",
			elephantine.LogKeyError, err)
		os.Exit(1)
	}

	updateCmd := cli.Command{
		Name:        "update",
		Description: "Refresh schema lockfile",
		Action:      updateAction,
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

	authFlags := []cli.Flag{
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
	}

	applyCmd.Flags = append(applyCmd.Flags, authFlags...)

	app := cli.App{
		Name:  "eleconf",
		Usage: "Elephant repository configuration tool",
		Commands: []*cli.Command{
			&updateCmd,
			&applyCmd,
		},
	}

	if err := app.Run(os.Args); err != nil {
		println("error: ", err.Error())
		os.Exit(1)
	}
}

func updateAction(c *cli.Context) error {
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
		return errors.New("missing lock file, run eleconf update")
	} else if err != nil {
		return fmt.Errorf("load lock file: %w", err)
	}

	var schemas []eleconf.LoadedSchema

	for _, set := range conf.SchemaSets {
		loaded, err := eleconf.LoadSchemaSet(ctx, set, lock, false)
		if err != nil {
			return fmt.Errorf("load schema set %q: %w",
				set.Name, err)
		}

		schemas = append(schemas, loaded...)
	}

	clients, err := getClients(ctx, c)
	if err != nil {
		return fmt.Errorf("get API clients: %w", err)
	}

	var changes []internal.ConfigurationChange

	scChanges, err := internal.GetSchemaChanges(
		ctx, clients, conf, schemas)
	if err != nil {
		return fmt.Errorf("calculate schema changes: %w", err)
	}

	changes = append(changes, scChanges...)

	mtChanges, err := internal.GetMetaTypeChanges(ctx, clients, conf)
	if err != nil {
		return fmt.Errorf("calculate meta type changes: %w", err)
	}

	changes = append(changes, mtChanges...)

	stChanges, err := internal.GetStatusChanges(ctx, clients, conf)
	if err != nil {
		return fmt.Errorf("calculate status changes: %w", err)
	}

	changes = append(changes, stChanges...)

	wfChanges, err := internal.GetWorkflowChanges(ctx, clients, conf)
	if err != nil {
		return fmt.Errorf("calculate workflow changes: %w", err)
	}

	changes = append(changes, wfChanges...)

	for _, change := range changes {
		op, info := change.Describe()

		col := color.New()

		switch op {
		case internal.OpAdd:
			col.Add(color.FgGreen)
		case internal.OpUpdate:
			col.Add(color.FgYellow)
		case internal.OpRemove:
			col.Add(color.FgRed)
		default:
			panic(fmt.Sprintf("unexpected internal.ChangeOp: %#v", op))
		}

		col.Printf("%s ", op)
		fmt.Println(info)

		warnCol := color.New(color.FgWhite, color.BgRed)

		w, ok := change.(doomsayer)
		if ok {
			for _, msg := range w.Warnings() {
				warnCol.Print(" Warning: ")
				fmt.Printf(" %s\n", msg)
			}
		}
	}

	println()

	if len(changes) == 0 {
		println("No changes needed")

		return nil
	}

	applyChanges := askForConfirmation(
		"Do you want to apply these changes?")
	if !applyChanges {
		return errors.New("aborted by user")
	}

	println()

	for _, change := range changes {
		op, info := change.Describe()

		info, _, _ = strings.Cut(info, "\n")
		info = strings.TrimRight(info, ":")

		fmt.Println(op, info)

		err := change.Execute(ctx, clients)
		if err != nil {
			return fmt.Errorf("failed to apply change: %w", err)
		}
	}

	println()
	println("Configuration has been updated")

	return nil
}

type doomsayer interface {
	Warnings() []string
}

func askForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			println(err.Error())

			return false
		}

		response = strings.ToLower(strings.TrimSpace(response))

		switch response {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}
