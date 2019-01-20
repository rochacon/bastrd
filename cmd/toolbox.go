package cmd

import (
	"archive/tar"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/rochacon/bastrd/pkg/user"

	"github.com/aws/aws-sdk-go/service/sts"
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

// toolboxSessionMain handles the user's MFA validation and toolbox initialization
// Overall steps:
// 1. AWS MFA validation and session token creation
// 2. Container setup (this is skipped on session resume)
// 3. AWS session token setup on container tmpfs mount (re-written on session resume to refresh expiration)
// 4. Attach to container
func toolboxSessionMain(ctx *cli.Context) (err error) {
	// duration := ctx.Int64("duration")
	image := ctx.String("image")
	sshArgs := ctx.String("c")
	username := ctx.String("username")
	if username == "" {
		return fmt.Errorf("username argument is required.")
	}
	// mfaToken := ctx.String("token")
	// sessionToken, err := getUserSessionToken(username, mfaToken, duration)
	// if err != nil {
	// 	return fmt.Errorf("error creating user %q session access keys: %s", username, err)
	// }
	log.Println("Opening session")
	err = ensureContainer(username, image, sshArgs)
	if err != nil {
		return fmt.Errorf("error opening session for user %q: %s", username, err)
	}
	// err = copyCredentialsToContainer(username, sessionToken)
	// if err != nil {
	// 	return fmt.Errorf("failed to copy credentials into session container for user %q: %s", username, err)
	// }
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
		fmt.Sprintf("--mount=type=bind,source=%s,destination=%s,bind-propagation=rprivate", usr.HomeDir(), usr.HomeDir()),
		"--user", fmt.Sprintf("%d:%d", usr.Uid(), usr.Uid()),
		"--workdir", usr.HomeDir(),
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

// copyCredentialsToContainer renders the awsCredentials template as
// /home/username/.aws/credentials file inside the toolbox
func copyCredentialsToContainer(username string, token *sts.Credentials) error {
	content := &bytes.Buffer{}
	err := awsCredentials.Execute(content, struct {
		AccessKeyId, Region, SecretAccessKey, SessionToken string
	}{
		AccessKeyId:     *token.AccessKeyId,
		SecretAccessKey: *token.SecretAccessKey,
		SessionToken:    *token.SessionToken,
		Region:          os.Getenv("AWS_DEFAULT_REGION"),
	})
	tarBuf := &bytes.Buffer{}
	w := tar.NewWriter(tarBuf)
	hdr := &tar.Header{
		Name: "credentials",
		Mode: 0600,
		Size: int64(content.Len()),
	}
	err = w.WriteHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write(content.Bytes())
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	// XXX(rochacon) can't copy as normal file since the target is the tmpfs mount
	cmd := exec.Command(DOCKER, "container", "exec", "-i", username, "tar", "vxC", "/home/"+username+"/.aws/")
	cmd.Stdin = tarBuf
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

// awsCredentials is a template to render user's ~/.aws/credentials file
var awsCredentials = template.Must(template.New("~/.aws/credentials").Parse(`
[default]
aws_access_key_id = {{ .AccessKeyId }}
aws_secret_access_key = {{ .SecretAccessKey }}
aws_session_token = {{ .SessionToken }}
{{ if .Region }}region = {{ .Region }}{{ end }}
`))
