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
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
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
	_, err := NewSessionCredentials(username, secretKey, mfaToken, 900*time.Second)
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

// getUserSessionToken creates a temporary Access Key to validate an user's MFA and retrieve a session token
func NewSessionCredentials(username, secretKey, mfaToken string, duration time.Duration) (*sts.Credentials, error) {
	iamSvc := iam.New(session.New())
	accessKeys, err := iamSvc.ListAccessKeys(&iam.ListAccessKeysInput{
		UserName: aws.String(username),
	})
	if err != nil {
		return nil, err
	}
	if len(accessKeys.AccessKeyMetadata) == 0 {
		return nil, fmt.Errorf("No matching access key found.")
	}
	accessKey := accessKeys.AccessKeyMetadata[0]
	if *accessKey.Status != iam.StatusTypeActive {
		return nil, fmt.Errorf("No active access key found.")
	}
	stsSvc := sts.New(session.New())
	accountID, err := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}
	mfaArn := fmt.Sprintf("arn:aws:iam::%s:mfa/%s", *accountID.Account, username)
	userSession, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(*accessKey.AccessKeyId, secretKey, ""),
	})
	stsSvc = sts.New(userSession)
	creds, err := stsSvc.GetSessionToken(&sts.GetSessionTokenInput{
		DurationSeconds: aws.Int64(int64(duration.Seconds())),
		SerialNumber:    aws.String(mfaArn),
		TokenCode:       aws.String(mfaToken),
	})
	if err != nil {
		return nil, fmt.Errorf("Error getting session token %q for %q: %s", mfaToken, username, err)
	}
	return creds.Credentials, nil
}
