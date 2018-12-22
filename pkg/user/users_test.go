package user

import (
	"testing"
)

func TestUsersDiff(t *testing.T) {
	users1 := Users{
		&User{Username: "rochacon"},
		&User{Username: "rodrigo.chacon"},
	}
	users2 := Users{
		&User{Username: "rochacon"},
	}
	diff := users1.Diff(users2)
	if len(diff) != 1 {
		t.Errorf("Unexpected diff length %d: %#v", len(diff), diff)
		t.Fail()
	}
	if diff[0].Username != "rodrigo.chacon" {
		t.Errorf("failed to diff Users collection, got %#v expected \"rodrigo.chacon\"", diff)
	}
}

func TestUsersDiffWhenEqual(t *testing.T) {
	users1 := Users{
		&User{Username: "rochacon"},
	}
	users2 := Users{
		&User{Username: "rochacon"},
	}
	diff := users1.Diff(users2)
	if len(diff) != 0 {
		t.Errorf("Unexpected diff length %d: %#v", len(diff), diff)
		t.Fail()
	}
}

func TestUserIn(t *testing.T) {
	users := Users{
		&User{Username: "rochacon"},
	}
	if userIn(User{Username: "rochacon"}, users) == false {
		t.Errorf("failed to find user \"rochacon\" on %#v", users)
	}
	if userIn(User{Username: "nope"}, users) == true {
		t.Errorf("user \"nope\" does not exist on collection, wrong match: %#v", users)
	}
}
