package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	osuser "os/user"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/urfave/cli"
)

const (
	PAM_FAIL    = 75
	PAM_NO_PERM = 77
)

var MFA = cli.Command{
	Name:    "mfa",
	Usage:   "Authenticate an user against an IAM role. This command is designed to be called by PAM with PAM_USER environment variable.",
	Action:  validateMFA,
	Aliases: []string{"validate-mfa", "validate_mfa"},
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "role-arn",
			Usage: "Role ARN for normal user.",
		},
		cli.StringFlag{
			Name:  "token",
			Usage: "AWS IAM MFA token.",
		},
	},
}

// validateMFA create a system user with a sandboxed shell
func validateMFA(ctx *cli.Context) error {
	username := os.Getenv("PAM_USER")
	if username == "" {
		return fmt.Errorf("Username argument (PAM_USER environment variable) is required.")
	}
	roleArn := ctx.String("role-arn")
	if roleArn == "" {
		return cli.NewExitError(fmt.Errorf("Role Arn must be configured."), PAM_FAIL)
	}
	mfaToken := ctx.String("token")
	if mfaToken == "" {
		fmt.Printf("Enter MFA code: ")
		reader := bufio.NewReader(os.Stdin)
		mfaToken, _ = reader.ReadString('\n')
		mfaToken = strings.TrimSpace(mfaToken)
	}
	if mfaToken == "" {
		return cli.NewExitError(fmt.Errorf("MFA code is required."), PAM_FAIL)
	}
	// validation session credentials last only 10s and are discarted
	_, err := NewRoleSessionCredentials(roleArn, username, mfaToken, 900*time.Second)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("Invalid credentials: %s", err), PAM_FAIL)
	}
	// check that user also exists on host
	if _, err := osuser.Lookup(username); err != nil {
		return cli.NewExitError(fmt.Errorf("User unavailable: %s", err), PAM_FAIL)
	}
	// user := &user.User{Username: username}
	// if err := user.Ensure(true); err != nil {
	// 	return cli.NewExitError(fmt.Errorf("failed to ensure user %q in the system: %s", username, err), PAM_FAIL)
	// }
	log.Printf("Authenticated user %q for AWS IAM role %q", username, roleArn)
	return nil
}

// getUserSessionToken creates a temporary Access Key to validate an user's MFA and retrieve a session token
func NewRoleSessionCredentials(roleArn, username, mfaToken string, duration time.Duration) (*sts.Credentials, error) {
	stsSvc := sts.New(session.New())
	accountID, err := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	timestamp := time.Now().Unix()
	sessionName := fmt.Sprintf("%s-%d-%s", username, timestamp, hostname)
	mfaArn := fmt.Sprintf("arn:aws:iam::%s:mfa/%s", *accountID.Account, username)
	creds, err := stsSvc.AssumeRole(&sts.AssumeRoleInput{
		DurationSeconds: aws.Int64(int64(duration.Seconds())),
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
		SerialNumber:    aws.String(mfaArn),
		TokenCode:       aws.String(mfaToken),
	})
	if err != nil {
		return nil, fmt.Errorf("Error getting session token %q for %q: %s", mfaToken, username, err)
	}
	return creds.Credentials, nil
}
