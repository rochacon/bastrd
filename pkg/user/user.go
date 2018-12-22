package user

import (
	"fmt"
	"log"
	"os/exec"
	osuser "os/user"
)

// User represents a mirrored user between AWS IAM and the local system
type User struct {
	ARN      string
	Groups   []*Group
	Username string
}

// Ensure ensure a user is correctly configured on the system
func (u *User) Ensure(sandboxed bool) error {
	return ensureUser(u.Username, sandboxed)
}

// Remove removes an user from the system
func (u *User) Remove() error {
	return exec.Command("/usr/sbin/userdel", "--remove", u.Username).Run()
}

// ensureUser add an user in the system idempotently
func ensureUser(username string, sandboxed bool) error {
	if userExists(username) {
		return nil
	}
	if err := userAdd(username, sandboxed); err != nil {
		return fmt.Errorf("failed to create user: %q", err)
	}
	log.Printf("Created user %q", username)
	return nil
}

// userAdd adds a user to the system with toolbox as the default shell
func userAdd(username string, sandboxed bool) error {
	shell := "/opt/bin/bastrd-toolbox"
	if !sandboxed {
		shell = "/bin/bash"
	}
	cmd := exec.Command(
		"/usr/sbin/useradd",
		"-m",
		"-G", "docker",
		"-s", shell,
		"-c", "bastrd managed user",
		username,
	)
	err := cmd.Run()
	if err != nil {
		out, _ := cmd.Output()
		log.Printf("call to useradd %q failed: %q %q", username, err, out)
		return fmt.Errorf("call to useradd %q failed: %q", username, err)
	}
	return nil
}

// userExists checks if the user already exists in the system
func userExists(username string) bool {
	_, err := osuser.Lookup(username)
	return err == nil
}
