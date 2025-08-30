package main

import (
	"context"
	"ki/src/cmd"
	"ki/src/config"
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
								Sources:  cli.EnvVars(config.ENV__USER_REGISTRY_PATH),
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
								Sources:  cli.EnvVars(config.ENV__USER_REGISTRY_PATH),
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
								Sources:  cli.EnvVars(config.ENV__USER_REGISTRY_PATH),
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
						Sources:  cli.EnvVars(config.ENV__BIND_ADDR),
						Required: false,
					},
					&cli.Int32Flag{
						Name:     "port",
						Aliases:  []string{"p"},
						Usage:    "The port number",
						Value:    9070,
						Sources:  cli.EnvVars(config.ENV__PORT),
						Required: false,
					},
					&cli.StringFlag{
						Name:     "registry",
						Aliases:  []string{"r"},
						Usage:    "The user registry file",
						Value:    "./users.json",
						Sources:  cli.EnvVars(config.ENV__USER_REGISTRY_PATH),
						Required: false,
					},
					&cli.Int64Flag{
						Name:     "max-upload-size",
						Aliases:  []string{"u"},
						Usage:    "The max number of bytes a file can be",
						Value:    config.MB * 512,
						Sources:  cli.EnvVars(config.ENV__MAX_UPLOAD_SIZE),
						Required: false,
					},
					&cli.StringSliceFlag{
						Name:    "trused-proxy",
						Aliases: []string{"P"},
						Usage:   "Subnet of trused-proxies",
						Value: []string{
							"127.0.0.0/8",
							"10.0.0.0/8",
							"172.16.0.0/12",
							"192.168.0.0/16",
							"fd00::/8",
							"::1/128",
						},
						Sources:  cli.EnvVars(config.ENV__TRUSTED_PROXIES),
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
