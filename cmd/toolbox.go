package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rochacon/bastrd/pkg/user"

	"github.com/google/shlex"
	"github.com/urfave/cli"
)

// Docker binary
const DOCKER = "/usr/bin/docker"

var Toolbox = cli.Command{
	Name:    "toolbox",
	Usage:   "Validates MFA and open a new authenticated toolbox session.",
	Action:  toolboxSessionMain,
	Aliases: []string{"session"},
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "c",
			Usage: "SSH command arguments.",
		},
		cli.Int64Flag{
			Name:  "duration",
			Usage: "Session credentials duration, in hours.",
			Value: 4,
		},
		cli.StringFlag{
			Name:  "image",
			Usage: "Sandbox container image.",
			Value: "docker.io/rochacon/bastrd-toolbox:latest",
		},
		cli.StringFlag{
			Name:  "token",
			Usage: "AWS IAM MFA token.",
		},
		cli.StringFlag{
			Name:  "username",
			Usage: "AWS IAM username for the sessioned.",
		},
	},
}

// toolboxSessionMain handles the user's toolbox initialization
// Overall steps:
// 1. Container setup (this is skipped on session resume)
// 2. Attach to container
func toolboxSessionMain(ctx *cli.Context) (err error) {
	// duration := ctx.Int64("duration")
	image := ctx.String("image")
	sshArgs := ctx.String("c")
	username := ctx.String("username")
	if username == "" {
		return fmt.Errorf("username argument is required.")
	}
	log.Println("Opening session")
	err = ensureContainer(username, image, sshArgs)
	if err != nil {
		return fmt.Errorf("error opening session for user %q: %s", username, err)
	}
	err = attachToContainer(username)
	if err != nil {
		return fmt.Errorf("failed to attach to session container for user %q: %s", username, err)
	}
	return err
}

// attachToContainer attaches current process std(out|in|err) to the container
func attachToContainer(username string) error {
	cmd := exec.Command(DOCKER, "container", "attach", username)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// ensureContainer ensure that user's toolbox container exists
func ensureContainer(username, image, command string) error {
	usr := &user.User{Username: username}
	containerID, err := exec.Command(DOCKER, "container", "ls", "--quiet", "--filter", "name="+usr.Username).Output()
	if err != nil {
		return fmt.Errorf("failed to check if container already running: %s", err)
	}
	// check if we got an ID
	if strings.TrimSpace(string(containerID)) != "" {
		return nil
	}
	// setup data directory for persistent storage
	os.MkdirAll(filepath.Join(usr.HomeDir(), "data"), 0750)
	// create container
	createArgs := []string{
		"container",
		"create",
		"--name", usr.Username,
		"--interactive",
		"--rm",
		"--tty",
		"--cap-drop=DAC_OVERRIDE",
		"--cap-drop=FOWNER",
		"--cap-drop=FSETID",
		"--cap-drop=MKNOD",
		"--cap-drop=NET_BIND_SERVICE",
		"--cap-drop=NET_RAW",
		"--cap-drop=SETFCAP",
		"--cap-drop=SETGID",
		"--cap-drop=SETPCAP",
		"--cap-drop=SETUID",
		"--cap-drop=SYS_CHROOT",
		"--env=HOME=" + usr.HomeDir(),
		"--env=USER=" + usr.Username,
		fmt.Sprintf("--mount=type=bind,source=/etc/group,destination=/etc/group,bind-propagation=rprivate,readonly"),
		fmt.Sprintf("--mount=type=bind,source=/etc/passwd,destination=/etc/passwd,bind-propagation=rprivate,readonly"),
		fmt.Sprintf("--mount=type=bind,source=%s/.aws,destination=%s/.aws,bind-propagation=rprivate,readonly", usr.HomeDir(), usr.HomeDir()),
		fmt.Sprintf("--mount=type=bind,source=%s/data,destination=%s/data,bind-propagation=rprivate", usr.HomeDir(), usr.HomeDir()),
		"--user", fmt.Sprintf("%d:%d", usr.Uid(), usr.Uid()),
		fmt.Sprintf("--workdir=%s/data", usr.HomeDir()),
	}
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock != "" {
		createArgs = append(createArgs, "--env=SSH_AUTH_SOCK="+sshAuthSock)
		createArgs = append(createArgs, fmt.Sprintf("--volume=%s:%s:ro", sshAuthSock, sshAuthSock))
	}
	createArgs = append(createArgs, image)
	if command != "" {
		commandSlice, _ := shlex.Split(command)
		createArgs = append(createArgs, commandSlice...)
	}
	cmd := exec.Command(DOCKER, createArgs...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create container: %s", err)
	}
	cmd = exec.Command(DOCKER, "container", "start", usr.Username)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
