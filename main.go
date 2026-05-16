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

	config.InitLogging()

	cmd := &cli.Command{
		Name:  "Ki",
		Usage: "A temporary file sharing service",
		Commands: []*cli.Command{
			{
				Name:        "registry",
				Usage:       "Commands for working with the user registry",
				Description: "The user registry is a simple JSON  file that contains the users who can upload files. Each user requires a username and password.",
				Commands: []*cli.Command{
					{
						Name:        "create",
						Usage:       "Create a new registry with the given users",
						Description: "Create a new user registry with the given users. If the file already exists, it will be overwritten.",
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
								Sources:  cli.EnvVars("KI_USER_REGISTRY_PATH"),
								Required: false,
							},
						},
					},
					{
						Name:        "validate",
						Usage:       "Parse and validate the registry",
						Description: "Parse and validate the registry",
						Action:      cmd.CmdValidateUserRegistry,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "file",
								Aliases:  []string{"o"},
								Usage:    "The source file",
								Value:    "./users.json",
								Sources:  cli.EnvVars("KI_USER_REGISTRY_PATH"),
								Required: false,
							},
						},
					},
					{
						Name:        "format",
						Usage:       "Parse, validate and format the registry",
						Description: "Validates the registry and formats it nicely",
						Action:      cmd.CmdFormatUserRegistry,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "file",
								Aliases:  []string{"o"},
								Usage:    "The source file",
								Value:    "./users.json",
								Sources:  cli.EnvVars("KI_USER_REGISTRY_PATH"),
								Required: false,
							},
						},
					},
				},
			},
			{
				Name:        "run",
				Usage:       "Run the server",
				Description: "Run the server",
				Action:      cmd.CmdServerMain,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "bind",
						Aliases:  []string{"b"},
						Usage:    "The bind address",
						Value:    "0.0.0.0",
						Sources:  cli.EnvVars("KI_BIND_ADDR"),
						Required: false,
					},
					&cli.Int32Flag{
						Name:     "port",
						Aliases:  []string{"p"},
						Usage:    "The port number",
						Value:    9070,
						Sources:  cli.EnvVars("KI_PORT"),
						Required: false,
					},
					&cli.StringFlag{
						Name:     "registry",
						Aliases:  []string{"r"},
						Usage:    "The user registry file",
						Value:    "./users.json",
						Sources:  cli.EnvVars("KI_USER_REGISTRY_PATH"),
						Required: false,
					},
					&cli.Int64Flag{
						Name:     "max-upload-size",
						Aliases:  []string{"u"},
						Usage:    "The max number of bytes a file can be",
						Value:    config.MaxUploadSize(),
						Sources:  cli.EnvVars("KI_MAX_UPLOAD_SIZE"),
						Required: false,
					},
					&cli.Int64Flag{
						Name:     "max-upload-chunk-size",
						Aliases:  []string{"c"},
						Usage:    "Uploads chunks will be limited to chunks of at most this size. The UI automatically chunks files larger than this value. This is mainly to bypass proxy payload limits (Cloudflare's 100mb free tier or nginx's 1mb default)",
						Value:    config.MaxChunkSize(),
						Sources:  cli.EnvVars("KI_MAX_UPLOAD_CHUNK_SIZE"),
						Required: false,
					},
					&cli.StringSliceFlag{
						Name:     "trusted-proxy",
						Aliases:  []string{"P"},
						Usage:    "Subnet of trused-proxies",
						Value:    []string{},
						Sources:  cli.EnvVars("KI_TRUSTED_PROXIES"),
						Required: false,
					},
					&cli.StringFlag{
						Name:     "file-dir",
						Aliases:  []string{"f"},
						Usage:    "The directory to store uploaded files",
						Value:    "./ki_files",
						Sources:  cli.EnvVars("KI_FILE_DIR"),
						Required: false,
					},
					&cli.StringFlag{
						Name:     "master-secret",
						Usage:    "The password used to encrypted metadata of files on disk",
						Sources:  cli.EnvVars("KI_MASTER_SECRET"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "tls-cert",
						Usage:    "The path to a TLS cert pair, both `tls-cert`.crt and `tls-cert`.key should exist",
						Sources:  cli.EnvVars("KI_TLS_CERT"),
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
