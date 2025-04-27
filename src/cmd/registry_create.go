package cmd

import (
	"context"
	"fmt"
	"ki/src/handlers/user"

	"github.com/urfave/cli/v3"
)

func CmdCreateUserRegistry(ctx context.Context, c *cli.Command) error {

	userStrs := c.Value("user").([]string)
	passStrs := c.Value("pass").([]string)
	file := c.Value("file").(string)

	if len(userStrs) != len(passStrs) {
		return fmt.Errorf("must have equal number of usernames and passwords")
	}

	reg := user.NewRegistry()

	for i, user := range userStrs {

		reg.AddUser(user, passStrs[i])
	}

	return reg.SaveToFile(file)
}
