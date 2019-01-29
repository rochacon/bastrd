package auth

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
)

// NewSessionCredentials creates time restrained credentials.
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
		return nil, fmt.Errorf("Error getting session token for %q with MFA %q: %s", username, mfaToken, err)
	}
	return creds.Credentials, nil
}
