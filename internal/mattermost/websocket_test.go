package mattermost

import "testing"

func TestMentionsCurrentUser(t *testing.T) {
	c := &Client{userID: "me"}
	if !c.mentionsCurrentUser(`["other","me"]`) {
		t.Fatal("expected user mention")
	}
	if c.mentionsCurrentUser(`["other"]`) {
		t.Fatal("unexpected mention")
	}
}
