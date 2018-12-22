package cmd

import (
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
)

// awsIAM interface holds required method signatures of IAM for easier test mocking
type awsIAM interface {
	CreateAccessKey(input *iam.CreateAccessKeyInput) (*iam.CreateAccessKeyOutput, error)
	DeleteAccessKey(input *iam.DeleteAccessKeyInput) (*iam.DeleteAccessKeyOutput, error)
	GetSSHPublicKey(input *iam.GetSSHPublicKeyInput) (*iam.GetSSHPublicKeyOutput, error)
	ListGroupsForUser(input *iam.ListGroupsForUserInput) (*iam.ListGroupsForUserOutput, error)
	ListSSHPublicKeys(input *iam.ListSSHPublicKeysInput) (*iam.ListSSHPublicKeysOutput, error)
}

// awsSTS interface holds required method signatures of STS for easier test mocking
type awsSTS interface {
	AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)
	GetCallerIdentity(input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
	GetSessionToken(input *sts.GetSessionTokenInput) (*sts.GetSessionTokenOutput, error)
}
