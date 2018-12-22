package user

import (
	"github.com/aws/aws-sdk-go/service/iam"
)

// IAM interface holds required method signatures of IAM for easier test mocking
type IAM interface {
	GetGroup(input *iam.GetGroupInput) (*iam.GetGroupOutput, error)
}
