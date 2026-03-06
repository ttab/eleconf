package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/ttab/clitools"
	"github.com/ttab/eleconf"
	"github.com/ttab/elephantine"
	"github.com/urfave/cli/v3"
)

const appName = "eleconf"

var version = "dev"

func main() {
	err := clitools.LoadEnv(appName)
	if err != nil {
		slog.Error("exiting: ",
			elephantine.LogKeyError, err)
		os.Exit(1)
	}

	versionCmd := cli.Command{
		Name: "version",
		Action: func(_ context.Context, _ *cli.Command) error {
			println(version)

			return nil
		},
	}

	updateCmd := cli.Command{
		Name:        "update",
		Description: "Refresh schema lockfile",
		Action:      updateAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      "dir",
				Usage:     "Configuration directory",
				Value:     ".",
				TakesFile: true,
			},
		},
	}

	authFlags := []cli.Flag{
		&cli.StringFlag{
			Name:    "env",
			Sources: cli.EnvVars("ENV"),
		},
		&cli.StringFlag{
			Name:    "client-id",
			Usage:   "Client ID",
			Sources: cli.EnvVars("CLIENT_ID"),
		},
		&cli.StringFlag{
			Name:    "client-secret",
			Usage:   "Client secret",
			Sources: cli.EnvVars("CLIENT_SECRET"),
		},
	}

	applyCmd := cli.Command{
		Name:        "apply",
		Description: "Applies elephant configuration",
		Action:      applyAction,
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:      "dir",
				Usage:     "Configuration directory",
				Value:     ".",
				TakesFile: true,
			},
		}, authFlags...),
	}

	app := &cli.Command{
		Name:  "eleconf",
		Usage: "Elephant repository configuration tool",
		Commands: []*cli.Command{
			&versionCmd,
			&updateCmd,
			&applyCmd,
			clitools.ConfigureCliCommands("eleconf", clitools.DefaultApplicationID),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		println("error: ", err.Error())
		os.Exit(1)
	}
}

func updateAction(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.String("dir")

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

func applyAction(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.String("dir")

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

	clients, err := getClients(ctx, cmd)
	if err != nil {
		return fmt.Errorf("get API clients: %w", err)
	}

	changes, err := eleconf.GetChanges(ctx, clients, conf, schemas)
	if err != nil {
		return fmt.Errorf("get changes: %w", err)
	}

	for _, change := range changes {
		op, info := change.Describe()

		col := color.New()

		switch op {
		case eleconf.OpAdd:
			col.Add(color.FgGreen)
		case eleconf.OpUpdate:
			col.Add(color.FgYellow)
		case eleconf.OpRemove:
			col.Add(color.FgRed)
		default:
			panic(fmt.Sprintf("unexpected eleconf.ChangeOp: %#v", op))
		}

		_, _ = col.Printf("%s ", op)
		fmt.Println(info)

		warnCol := color.New(color.FgWhite, color.BgRed)

		w, ok := change.(doomsayer)
		if ok {
			for _, msg := range w.Warnings() {
				_, _ = warnCol.Print(" Warning: ")
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
