package user

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"strings"
)

// User represents a mirrored user between AWS IAM and the local system
type User struct {
	Groups   []*Group
	Username string
}

// Ensure ensure a user is correctly configured on the system
func (u *User) Ensure(sandboxed bool, additionalGroups []string) error {
	return ensureUser(u.Username, sandboxed, additionalGroups)
}

// HomeDir returns the user's home directory
func (u User) HomeDir() string {
	return filepath.Join("/home", u.Username)
}

// Remove removes an user from the system
func (u *User) Remove() error {
	return exec.Command("/usr/sbin/userdel", "--remove", u.Username).Run()
}

// Uid returns the user unique id
func (u User) Uid() uint16 {
	return uidFromString(u.Username)
}

// ensureUser add an user in the system idempotently
func ensureUser(username string, sandboxed bool, additionalGroups []string) error {
	if userExists(username) {
		return nil
	}
	if err := userAdd(username, sandboxed, additionalGroups); err != nil {
		return fmt.Errorf("failed to create user: %q", err)
	}
	log.Printf("Created user %q", username)
	return nil
}

// userAdd adds a user to the system with toolbox as the default shell
func userAdd(username string, sandboxed bool, additionalGroups []string) error {
	shell := "/opt/bin/bastrd-toolbox"
	if !sandboxed {
		shell = "/bin/bash"
	}
	uid := fmt.Sprintf("%d", uidFromString(username))
	cmd := exec.Command(
		"/usr/sbin/useradd",
		"-m",
		"-u", uid,
		"-U",
		"-G", strings.Join(additionalGroups, ","),
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

// uidFromString Converts an string into an uid
func uidFromString(awsID string) uint16 {
	sha := sha1.Sum([]byte(awsID))
	last2 := sha[len(sha)-2:]
	n := binary.LittleEndian.Uint16(last2)
	return 2000 + (n / 2)
}
