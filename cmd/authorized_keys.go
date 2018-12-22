package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/urfave/cli"
)

var AuthorizedKeys = cli.Command{
	Name:      "authorized-keys",
	Usage:     "List AWS IAM user registered SSH public keys.",
	ArgsUsage: "username",
	Action:    getAuthorizedKeysForUser,
	Aliases:   []string{"authorized_keys"},
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "allowed-groups",
			Usage: "Comma separated list of AWS IAM Groups allowed to SSH. (defaults to bastrd)",
		},
	},
}

// getAuthorizedKeysForUser retrieves
func getAuthorizedKeysForUser(ctx *cli.Context) error {
	username := ctx.Args().Get(0)
	if username == "" {
		return fmt.Errorf("Username argument is required.")
	}
	allowedGroups := ctx.StringSlice("allowed-groups")
	if len(allowedGroups) == 0 {
		allowedGroups = append(allowedGroups, "bastrd")
	}

	awsSession := session.Must(session.NewSession(&aws.Config{}))
	iamSvc := iam.New(awsSession)

	if !userBelongsToAllowedGroups(iamSvc, username, allowedGroups) {
		return fmt.Errorf("User %q is not allowed to SSH into this instance, this incident will be reported.", username)
	}

	keys, err := getUserSSHPublicKeys(iamSvc, username)
	if err != nil {
		return fmt.Errorf("Error while retrieving user SSH public keys for user %q: %s", username, err)
	}
	if len(keys) == 0 {
		return fmt.Errorf("Found no SSH public keys for user %q.", username)
	}
	fmt.Println(strings.Join(keys, "\n"))
	return nil
}

// getUserSSHPublicKeys retrieves AWS IAM user SSH public keys
func getUserSSHPublicKeys(iamSvc awsIAM, username string) ([]string, error) {
	keys := []string{}
	sshKeys, err := iamSvc.ListSSHPublicKeys(&iam.ListSSHPublicKeysInput{
		UserName: aws.String(username),
	})
	if err != nil {
		return keys, err
	}
	for _, key := range sshKeys.SSHPublicKeys {
		if *key.Status != iam.StatusTypeActive {
			log.Printf("authorized-keys: skipping key %q, status %q", *key.SSHPublicKeyId, *key.Status)
			continue
		}
		k, err := iamSvc.GetSSHPublicKey(&iam.GetSSHPublicKeyInput{
			Encoding:       aws.String(iam.EncodingTypeSsh),
			SSHPublicKeyId: key.SSHPublicKeyId,
			UserName:       aws.String(username),
		})
		if err != nil {
			return keys, err
		}
		keys = append(keys, *k.SSHPublicKey.SSHPublicKeyBody)
	}
	return keys, nil
}

// userBelongsToAllowedGroups checks wether user is a member of SSH allowed groups
func userBelongsToAllowedGroups(iamSvc awsIAM, username string, allowedGroups []string) bool {
	userGroups, err := iamSvc.ListGroupsForUser(&iam.ListGroupsForUserInput{
		UserName: aws.String(username),
	})
	if err != nil {
		log.Println("authorized-keys: iam.ListGroupsForUser returned error:", err)
		return false
	}
	for _, group := range userGroups.Groups {
		if stringIn(*group.GroupName, allowedGroups) {
			// log.Printf("authorized-keys: user %q belongs to allowed group %q", username, *group.GroupName)
			return true
		}
	}
	return false
}

// stringIn matches if a string exist in a string slice
func stringIn(s string, ss []string) bool {
	for _, item := range ss {
		if s == item {
			return true
		}
	}
	return false
}
