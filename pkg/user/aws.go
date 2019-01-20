package user

import (
	"github.com/aws/aws-sdk-go/service/iam"
)

// IAM interface holds required method signatures of IAM for easier test mocking
type IAM interface {
	GetGroup(input *iam.GetGroupInput) (*iam.GetGroupOutput, error)
	GetSSHPublicKey(input *iam.GetSSHPublicKeyInput) (*iam.GetSSHPublicKeyOutput, error)
	ListGroupsForUser(input *iam.ListGroupsForUserInput) (*iam.ListGroupsForUserOutput, error)
	ListSSHPublicKeys(input *iam.ListSSHPublicKeysInput) (*iam.ListSSHPublicKeysOutput, error)
}
