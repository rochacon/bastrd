package user

import (
	"fmt"
	"os/exec"
)

// Group holds a user group metadata
type Group struct {
	Name string
}

// Ensure a group is configured in the system
func (g *Group) Ensure() error {
	cmd := exec.Command("/usr/sbin/groupadd", "-f", g.Name)
	err := cmd.Run()
	if err != nil {
		out, _ := cmd.Output()
		return fmt.Errorf("call to groupadd %q failed: %q %q", g.Name, err, out)
	}
	return nil
}

// EnsureUser ensures an user is member of a system group
func (g *Group) EnsureUser(user *User) error {
	cmd := exec.Command("/usr/sbin/usermod", "-a", "-G", g.Name, user.Username)
	err := cmd.Run()
	if err != nil {
		out, _ := cmd.Output()
		return fmt.Errorf("failed to add user %q to group %q: %q %q", user.Username, g.Name, err, out)
	}
	return nil
}

// RemoveUser removes an user from the group
func (g *Group) RemoveUser(user *User) error {
	cmd := exec.Command("gpasswd", "-d", user.Username, g.Name)
	err := cmd.Run()
	if err != nil {
		out, _ := cmd.Output()
		return fmt.Errorf("failed to remove user %q from group %q: %q %q", user.Username, g.Name, err, out)
	}
	return nil
}
