package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	osuser "os/user"
	"strings"
	"time"

	"github.com/rochacon/bastrd/pkg/auth"

	"github.com/urfave/cli"
)

var PAM = cli.Command{
	Name:   "pam",
	Usage:  "Authenticate an user against an IAM role. This command is designed to be called by PAM pam_exec module.",
	Action: pamMain,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:   "username",
			Usage:  "AWS IAM username.",
			EnvVar: "PAM_USER",
		},
	},
}

// pamMain
func pamMain(ctx *cli.Context) error {
	username := ctx.String("username")
	if username == "" {
		return fmt.Errorf("Username argument (PAM_USER environment variable) is required.")
	}
	reader := bufio.NewReader(os.Stdin)
	secretKey, _ := reader.ReadString('\n')
	secretKey = strings.TrimSpace(secretKey)
	secretKey = strings.Trim(secretKey, "\x00")
	lenSecretKey := len(secretKey)
	if secretKey == "" || lenSecretKey < 6 {
		return cli.NewExitError(fmt.Errorf("Secret Key + MFA core is required."), 1)
	}
	secretKey, mfaToken := secretKey[:lenSecretKey-6], secretKey[lenSecretKey-6:]
	// validation session credentials last only 10s and are discarted
	_, err := auth.NewSessionCredentials(username, secretKey, mfaToken, 900*time.Second)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("Invalid credentials: %s", err), 1)
	}
	// check that user also exists on host
	if _, err := osuser.Lookup(username); err != nil {
		return cli.NewExitError(fmt.Errorf("User unavailable: %s", err), 1)
	}
	// TODO setup user owned tmpfs for ~/.aws/credentials
	log.Printf("Authenticated user %q", username)
	return nil
}
