package cmd

import (
	"context"
	"ki/src/handlers/user"

	"github.com/urfave/cli/v3"
)

func CmdValidateUserRegistry(ctx context.Context, c *cli.Command) error {

	file := c.Value("file").(string)

	reg := user.NewRegistry()

	if err := reg.LoadFromFile(file); err != nil {
		return err
	}

	return nil
}
