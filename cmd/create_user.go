package cmd

import (
	"fmt"
	"log"
	"os"
	osuser "os/user"

	"github.com/rochacon/bastrd/pkg/user"

	"github.com/urfave/cli"
)

const (
	PAM_FAIL    = 75
	PAM_NO_PERM = 77
)

var CreateUser = cli.Command{
	Name:    "create-user",
	Usage:   "WIP: Create sandboxed user. This command is designed to be called by PAM with PAM_USER environment variable.",
	Action:  createUser,
	Aliases: []string{"create_user"},
}

// createUser create a system user with a sandboxed shell
func createUser(ctx *cli.Context) error {
	username := os.Getenv("PAM_USER")
	if username == "" {
		return fmt.Errorf("Username argument (PAM_USER environment variable) is required.")
	}
	if _, err := osuser.Lookup(username); err == nil {
		return nil
	}
	user := &user.User{Username: username}
	if err := user.Ensure(true); err != nil {
		return cli.NewExitError(fmt.Errorf("failed to ensure user %q in the system: %s", username, err), PAM_FAIL)
	}
	log.Printf("Imported user %q from AWS IAM", username)
	return nil
}
