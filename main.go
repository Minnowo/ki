package main

import (
	"context"
	"ki/src/cmd"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func main() {

	cmd := &cli.Command{
		Name:  "Ki",
		Usage: "A secure upload portal",
		Commands: []*cli.Command{
			{
				Name:        "registry",
				Description: "Commands for working with the user registry",
				Commands: []*cli.Command{
					{
						Name:        "create",
						Description: "Create a new user registry",
						Action:      cmd.CmdCreateUserRegistry,
						Flags: []cli.Flag{
							&cli.StringSliceFlag{
								Name:     "user",
								Aliases:  []string{"u"},
								Usage:    "Create a user with this name",
								Required: false,
							},
							&cli.StringSliceFlag{
								Name:     "pass",
								Aliases:  []string{"p"},
								Usage:    "Create a user with this password",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "file",
								Aliases:  []string{"o"},
								Usage:    "The output file",
								Value:    "./users.json",
								Sources:  cli.EnvVars("USER_REGISTRY_PATH"),
								Required: false,
							},
						},
					},
					{
						Name:        "validate",
						Description: "Parse and validate the registry",
						Action:      cmd.CmdValidateUserRegistry,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "file",
								Aliases:  []string{"o"},
								Usage:    "The source file",
								Value:    "./users.json",
								Sources:  cli.EnvVars("USER_REGISTRY_PATH"),
								Required: false,
							},
						},
					},
					{
						Name:        "format",
						Description: "Validates the registry and formats it nicely",
						Action:      cmd.CmdFormatUserRegistry,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "file",
								Aliases:  []string{"o"},
								Usage:    "The source file",
								Value:    "./users.json",
								Sources:  cli.EnvVars("USER_REGISTRY_PATH"),
								Required: false,
							},
						},
					},
				},
			},
			{
				Name:        "run",
				Description: "Run the server",
				Action:      cmd.CmdServerMain,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "bind",
						Aliases:  []string{"b"},
						Usage:    "The bind address",
						Value:    "0.0.0.0",
						Sources:  cli.EnvVars("BIND_ADDR"),
						Required: false,
					},
					&cli.Int32Flag{
						Name:     "port",
						Aliases:  []string{"p"},
						Usage:    "The port number",
						Value:    9070,
						Sources:  cli.EnvVars("PORT"),
						Required: false,
					},
					&cli.StringFlag{
						Name:     "registry",
						Aliases:  []string{"r"},
						Usage:    "The user registry file",
						Value:    "./users.json",
						Sources:  cli.EnvVars("USER_REGISTRY_PATH"),
						Required: false,
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Msg("")
	}

}
