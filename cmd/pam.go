package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	osuser "os/user"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/rochacon/bastrd/pkg/auth"

	"github.com/aws/aws-sdk-go/service/sts"
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
		cli.BoolFlag{
			Name:  "skip-credential-update",
			Usage: "Skip session credential update.",
			Value: false,
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
	creds, err := auth.NewSessionCredentials(username, secretKey, mfaToken, 900*time.Second)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("Invalid credentials: %s", err), 1)
	}
	// check that user also exists on host
	usr, err := osuser.Lookup(username)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("User unavailable: %s", err), 1)
	}
	// setup user session credentials
	// FIXME setup user owned tmpfs for ~/.aws/credentials
	if ctx.Bool("skip-credential-update") == false {
		os.MkdirAll(filepath.Join(usr.HomeDir, ".aws"), 0700)
		err = renderUserSessionCredentials(filepath.Join(usr.HomeDir, ".aws", "credentials"), creds)
		if err != nil {
			log.Printf("Failed to set session credentials: %s", err)
		}
	}
	log.Printf("Authenticated user %q", username)
	return nil
}

// renderUserSessionCredentials renders the awsCredentials template as
// /home/username/.aws/credentials file inside the toolbox
func renderUserSessionCredentials(filename string, token *sts.Credentials) error {
	fp, err := os.Open(filename)
	if err != nil {
		return err
	}
	err = awsCredentials.Execute(fp, struct {
		AccessKeyId, Region, SecretAccessKey, SessionToken string
	}{
		AccessKeyId:     *token.AccessKeyId,
		SecretAccessKey: *token.SecretAccessKey,
		SessionToken:    *token.SessionToken,
		Region:          os.Getenv("AWS_DEFAULT_REGION"),
	})
	if err != nil {
		return err
	}
	defer fp.Close()
	return fp.Chmod(0600)
}

// awsCredentials is a template to render user's ~/.aws/credentials file
var awsCredentials = template.Must(template.New("~/.aws/credentials").Parse(`
[default]
aws_access_key_id = {{ .AccessKeyId }}
aws_secret_access_key = {{ .SecretAccessKey }}
aws_session_token = {{ .SessionToken }}
{{ if .Region }}region = {{ .Region }}{{ end }}
`))
