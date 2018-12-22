package user

import (
	"fmt"
	"log"
	"os/exec"
	osuser "os/user"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
)

// Users holds a collection of User
type Users []*User

// Diff returns the difference between self and another Users collection
func (self Users) Diff(users Users) Users {
	diff := Users{}
	for _, u := range self {
		if !userIn(*u, users) {
			diff = append(diff, u)
		}
	}
	return diff
}

// FromIAMGroups returns a single Users collection for the given AWS IAM groups
func FromIAMGroups(svc IAM, groups ...*Group) (Users, error) {
	users := Users{}
	usersMap := map[string]*User{}
	for _, group := range groups {
		log.Printf("Retrieving AWS IAM group %q", group.Name)
		iamGroup, err := svc.GetGroup(&iam.GetGroupInput{
			GroupName: aws.String(group.Name),
		})
		if err != nil {
			return users, fmt.Errorf("Error retrieving group %q info: %s", group.Name, err)
		}
		for _, iamUser := range iamGroup.Users {
			if *iamUser.UserName == "root" || *iamUser.UserName == "core" || *iamUser.UserName == "ec2-user" {
				log.Printf("Found reserved username %q in group %q, skipping it.", *iamUser.UserName, *iamGroup.Group.GroupName)
				continue
			}
			usr, ok := usersMap[*iamUser.UserName]
			if !ok {
				usr = &User{Username: *iamUser.UserName}
				usr.Groups = []*Group{group}
				users = append(users, usr)
			}
			usr.Groups = append(usr.Groups, group)
		}
	}
	return users, nil
}

// FromSystemGroups returns a single Users collection for the given system groups
func FromSystemGroups(groups ...*Group) (Users, error) {
	users := Users{}
	usersMap := map[string]*User{}
	for _, group := range groups {
		_, err := osuser.LookupGroup(group.Name)
		if err != nil {
			if _, ok := err.(osuser.UnknownGroupError); !ok {
				return users, err
			}
			continue
		}
		groupLine, err := exec.Command("getent", "group", group.Name).Output()
		if err != nil {
			return users, fmt.Errorf("failed to retrieve group details: %s", err)
		}
		parts := strings.Split(strings.TrimSpace(string(groupLine)), ":")
		usersPart := strings.Split(parts[3], ",")
		for _, username := range usersPart {
			if username == "" {
				continue
			}
			if username == "root" || username == "core" || username == "ec2-user" {
				log.Printf("Found reserved username %q in group %q, skipping it.", username, group.Name)
				continue
			}
			usr, ok := usersMap[username]
			if !ok {
				usr = &User{Username: username}
				usr.Groups = []*Group{group}
				users = append(users, usr)
			}
			usr.Groups = append(usr.Groups, group)
		}
	}
	return users, nil
}

// userIn checks wether an User exists in a Users collection
func userIn(user User, users Users) bool {
	for _, u := range users {
		if user.Username == u.Username {
			return true
		}
	}
	return false
}
