package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ttab/clitools"
	"github.com/ttab/eleconf"
	"github.com/ttab/elephant-api/repository"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"
)

var _ eleconf.Clients = &clients{}

type clients struct {
	Workflows repository.Workflows
	Schemas   repository.Schemas
	Metrics   repository.Metrics
}

// GetMetrics implements eleconf.Clients.
func (c *clients) GetMetrics() repository.Metrics {
	return c.Metrics
}

// GetSchemas implements eleconf.Clients.
func (c *clients) GetSchemas() repository.Schemas {
	return c.Schemas
}

// GetWorkflows implements eleconf.Clients.
func (c *clients) GetWorkflows() repository.Workflows {
	return c.Workflows
}

func getClients(
	ctx context.Context,
	c *cli.Context,
) (*clients, error) {
	clientID := c.String("client-id")
	clientSecret := c.String("client-secret")
	env := c.String("env")

	if clientID == "" {
		clientID = clitools.DefaultApplicationID
	}

	conf, err := clitools.NewConfigurationHandler(
		appName, clientID, env,
	)
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}

	endpoint, ok := conf.GetEndpoint("repository")
	if !ok {
		return nil, errors.New(
			"no repository endpoint configured for environment")
	}

	var token oauth2.TokenSource

	// Including doc_read here lets unprivileged client check if the state
	// is clean.
	scopes := []string{
		"workflow_admin", "schema_admin", "doc_read",
		"metrics_admin",
	}

	if clientSecret != "" {
		t, err := conf.GetClientAccessToken(
			ctx, clientID, clientSecret, scopes)
		if err != nil {
			return nil, fmt.Errorf(
				"get client access token: %w", err)
		}

		token = t
	} else {
		t, err := conf.GetAccessToken(ctx, scopes)
		if err != nil {
			return nil, fmt.Errorf("get access token: %w", err)
		}

		err = conf.Save()
		if err != nil {
			slog.Warn("failed to save configuration", "err", err)
		}

		token = oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: t.Token,
		})
	}

	client := oauth2.NewClient(ctx, token)

	return &clients{
		Workflows: repository.NewWorkflowsProtobufClient(endpoint, client),
		Schemas:   repository.NewSchemasProtobufClient(endpoint, client),
		Metrics:   repository.NewMetricsProtobufClient(endpoint, client),
	}, nil
}
