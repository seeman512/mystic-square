package model

import (
	"encoding/json"
	"testing"
)

func TestUserRoles(t *testing.T) {
	admin := User{FullName: "Ada Lovelace", Role: UserRoleAdmin}
	if !admin.Valid() || !admin.CanManageLevels() {
		t.Fatal("admin should be valid and allowed to manage levels")
	}

	player := User{FullName: "Grace Hopper", Role: UserRoleUser}
	if !player.Valid() || player.CanManageLevels() {
		t.Fatal("user should be valid and not allowed to manage levels")
	}

	if (User{FullName: "Unknown", Role: UserRole("moderator")}).Valid() {
		t.Fatal("unsupported role should be invalid")
	}
}

func TestGameJSONShape(t *testing.T) {
	game := Game{
		LevelID: 3,
		UserID:  7,
		Init:    []int{1, 2, 3, 0},
		Moves:   []Move{{4, 2}, {4, 3}},
	}
	data, err := json.Marshal(game)
	if err != nil {
		t.Fatalf("marshal game: %v", err)
	}
	if got, want := string(data), `{"id":0,"level_id":3,"user_id":7,"init":[1,2,3,0],"moves":[[4,2],[4,3]],"is_win":false}`; got != want {
		t.Fatalf("game JSON = %s, want %s", got, want)
	}
}
