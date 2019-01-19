package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rochacon/bastrd/pkg/user"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/urfave/cli"
)

var (
	defaultAdditionalGroups = cli.StringSlice([]string{"docker"})
)

var Sync = cli.Command{
	Name:    "sync",
	Usage:   "Sync AWS IAM users.",
	Action:  syncMain,
	Aliases: []string{"sync-users", "sync_users"},
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "additional-groups",
			Usage: "System user additional groups.",
			Value: &defaultAdditionalGroups,
		},
		cli.BoolFlag{
			Name:  "disable-sandbox",
			Usage: "Disable users sandboxed sessions.",
		},
		cli.StringSliceFlag{
			Name:  "groups",
			Usage: "AWS IAM group names to be synced. ATTENTION: Make sure these groups names don't conflict with existent system groups.",
		},
		cli.DurationFlag{
			Name:  "interval",
			Usage: "Time interval between sync loops.",
		},
	},
}

func syncMain(ctx *cli.Context) error {
	groups := ctx.StringSlice("groups")
	if len(groups) == 0 {
		return fmt.Errorf("You must provide at least 1 AWS IAM group name.")
	}
	interval := ctx.Duration("interval")
	if interval.Minutes() == 0.0 && interval.Seconds() == 0.0 {
		log.Println("Defaulting interval to 1m")
		interval = time.Second * 60
	}
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	log.Println("Executing initial sync")
	err := syncGroupsUsers(ctx)
	if err != nil {
		log.Printf("initial sync failed: %s", err)
	}
	log.Printf("Initiating sync loop for groups: %s", strings.Join(groups, ", "))
	for {
		select {
		case <-time.After(interval):
			log.Printf("Starting sync")
			err = syncGroupsUsers(ctx)
			if err != nil {
				return err
			}
			log.Printf("Finished sync")
		case <-quit:
			log.Println("Received SIGINT, quitting.")
			return nil
		}
	}
}

// syncGroupsUsers synchronizes users from AWS IAM
func syncGroupsUsers(ctx *cli.Context) error {
	additionalGroups := ctx.StringSlice("additional-groups")
	isSandboxed := ctx.Bool("disable-sandbox") == false
	groupNames := ctx.StringSlice("groups")
	groups := []*user.Group{}
	for _, name := range groupNames {
		groups = append(groups, &user.Group{Name: name})
	}

	awsSession := session.Must(session.NewSession(&aws.Config{}))
	iamSvc := iam.New(awsSession)

	iamUsers, err := user.FromIAMGroups(iamSvc, groups...)
	if err != nil {
		return fmt.Errorf("failed to retrieve AWS IAM users list: %s", err)
	}
	sysUsers, err := user.FromSystemGroups(groups...)
	if err != nil {
		return fmt.Errorf("failed to retrieve system users list: %s", err)
	}

	// Ensure groups in the system
	for _, group := range groups {
		log.Printf("Ensuring group %q", group.Name)
		err = group.Ensure()
		if err != nil {
			log.Printf("Failed to ensure group %q in the system: %s", group.Name, err)
			continue
		}
	}

	// create AWS IAM users that do not exist in the system
	for _, u := range iamUsers.Diff(sysUsers) {
		log.Printf("Ensuring user %q", u.Username)
		err = u.Ensure(isSandboxed, additionalGroups)
		if err != nil {
			log.Printf("Failed to ensure user %q in the system: %s", u.Username, err)
			continue
		}
		for _, g := range u.Groups {
			log.Printf("Ensuring user %q in group %q", u.Username, g.Name)
			err = g.EnsureUser(u)
			if err != nil {
				log.Printf("Failed to ensure user %q in the system group %q: %s", u.Username, g.Name, err)
				continue
			}
		}
	}

	// remove system users that aren't on AWS IAM anymore
	for _, u := range sysUsers.Diff(iamUsers) {
		log.Printf("Removing user %q from the system", u.Username)
		err = u.Remove()
		if err != nil {
			log.Printf("Failed to remove user %q from the system: %s", u.Username, err)
			continue
		}
	}
	return err
}
