package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ttab/clitools"
	"github.com/ttab/eleconf/internal"
	"github.com/ttab/elephant-api/repository"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"
)

var _ internal.Clients = &clients{}

type clients struct {
	Workflows repository.Workflows
	Schemas   repository.Schemas
}

// GetSchemas implements internal.Clients.
func (c *clients) GetSchemas() repository.Schemas {
	return c.Schemas
}

// GetWorkflows implements internal.Clients.
func (c *clients) GetWorkflows() repository.Workflows {
	return c.Workflows
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
