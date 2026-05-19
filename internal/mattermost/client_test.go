package mattermost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"band-tui/internal/config"
)

func TestClientConnectLoadAndSend(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/users/me", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("auth header = %q", got)
		}
		_ = json.NewEncoder(w).Encode(mmUser{ID: "u1", Username: "sasha", FirstName: "Sasha", LastName: "Neupokoev"})
	})
	mux.HandleFunc("/api/v4/users/u1/teams", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]mmTeam{{ID: "t1", Name: "team", DisplayName: "Team"}})
	})
	mux.HandleFunc("/api/v4/users/u1/teams/t1/channels", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]mmChannel{{ID: "c1", TeamID: "t1", Name: "town", DisplayName: "Town", Type: "O", Header: "Town topic", MemberCount: 42, TotalMsgCount: 10}})
	})
	mux.HandleFunc("/api/v4/users/u1/teams/t1/channels/members", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]mmChannelMember{{ChannelID: "c1", MsgCount: 7, MentionCount: 2, LastViewedAt: 2}})
	})
	mux.HandleFunc("/api/v4/users/u1/channels", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]mmChannel{
			{ID: "c1", TeamID: "t1", Name: "town", DisplayName: "Town", Type: "O", TotalMsgCount: 10},
			{ID: "d1", Name: "u1__u2", Type: "D", TotalMsgCount: 5, LastPostAt: 200},
			{ID: "d2", Name: "u1__u3", Type: "D", TotalMsgCount: 5, LastPostAt: 100},
		})
	})
	mux.HandleFunc("/api/v4/users/u1/channels/members", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]mmChannelMember{{ChannelID: "d1", MsgCount: 4}, {ChannelID: "d2", MsgCount: 5}})
	})
	mux.HandleFunc("/api/v4/users/status/ids", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]mmStatus{{UserID: "u2", Status: "online"}, {UserID: "u3", Status: "offline"}})
	})
	mux.HandleFunc("/api/v4/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("in_channel") {
		case "d1":
			_ = json.NewEncoder(w).Encode([]mmUser{{ID: "u1", Username: "sasha", FirstName: "Sasha", LastName: "Neupokoev"}, {ID: "u2", Username: "alice", FirstName: "Alice", LastName: "Smith"}})
		case "d2":
			_ = json.NewEncoder(w).Encode([]mmUser{{ID: "u1", Username: "sasha", FirstName: "Sasha", LastName: "Neupokoev"}, {ID: "u3", Username: "bob", FirstName: "Bob", LastName: "Brown"}})
		default:
			t.Fatalf("unexpected users query: %s", r.URL.RawQuery)
		}
	})
	mux.HandleFunc("/api/v4/channels/c1/posts", func(w http.ResponseWriter, r *http.Request) {
		if before := r.URL.Query().Get("before"); before == "p1" {
			_ = json.NewEncoder(w).Encode(mmPostList{
				Order: []string{"p0"},
				Posts: map[string]mmPost{"p0": {ID: "p0", ChannelID: "c1", UserID: "u2", Message: "zero", CreateAt: 0}},
			})
			return
		} else if before != "" {
			t.Fatalf("unexpected before query: %q", before)
		}
		_ = json.NewEncoder(w).Encode(mmPostList{
			Order: []string{"r2", "p2", "p1"},
			Posts: map[string]mmPost{
				"p1": {ID: "p1", ChannelID: "c1", UserID: "u1", Message: "one", CreateAt: 1},
				"p2": {ID: "p2", ChannelID: "c1", UserID: "u2", Message: "two", CreateAt: 2, FileIDs: []string{"f1"}, Metadata: &mmPostMetadata{Reactions: []mmReaction{{UserID: "u1", PostID: "p2", EmojiName: "+1"}}, Files: []mmFileInfo{{ID: "f1", Name: "photo.png", MIMEType: "image/png", Size: 2048, Width: 640, Height: 480}}}},
				"r2": {ID: "r2", ChannelID: "c1", RootID: "p1", UserID: "u2", Message: "thread reply", CreateAt: 3},
			},
		})
	})
	mux.HandleFunc("/api/v4/posts/p1/thread", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mmPostList{
			Order: []string{"r2", "r1", "p1"},
			Posts: map[string]mmPost{
				"p1": {ID: "p1", ChannelID: "c1", UserID: "u1", Message: "one", CreateAt: 1},
				"r1": {ID: "r1", ChannelID: "c1", RootID: "p1", UserID: "u2", Message: "reply 1", CreateAt: 2, Metadata: &mmPostMetadata{Reactions: []mmReaction{{UserID: "u1", PostID: "r1", EmojiName: "heart"}}}},
				"r2": {ID: "r2", ChannelID: "c1", RootID: "p1", UserID: "u2", Message: "reply 2", CreateAt: 3},
			},
		})
	})
	mux.HandleFunc("/api/v4/users/ids", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]mmUser{{ID: "u2", Username: "alice", FirstName: "Alice", LastName: "Smith"}})
	})
	mux.HandleFunc("/api/v4/channels/members/u1/view", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("view method = %s", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["channel_id"] != "c1" {
			t.Fatalf("view channel_id = %q", body["channel_id"])
		}
		if _, ok := body["prev_channel_id"]; !ok {
			t.Fatalf("view missing prev_channel_id: %#v", body)
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v4/posts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(mmPost{ID: "p3", ChannelID: "c1", UserID: "u1", Message: "sent", CreateAt: 3})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(config.Config{ServerURL: server.URL, Token: "token"})
	session, err := client.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if session.User.Username != "sasha" || session.User.DisplayName != "Sasha Neupokoev" || len(session.Teams) != 1 {
		t.Fatalf("bad session: %#v", session)
	}
	channels, err := client.LoadChannels(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 3 || channels[0].ID != "d1" || channels[0].DisplayName != "Alice Smith" || channels[0].Unread != 1 || channels[0].LastPostAt != 200 || channels[0].Status != "online" || channels[0].MemberCount != 1 || channels[1].ID != "d2" || channels[1].Status != "offline" || channels[2].ID != "c1" || channels[2].Unread != 3 || channels[2].Mentions != 2 || channels[2].LastViewedAt != 2 || channels[2].Header != "Town topic" || channels[2].MemberCount != 42 {
		t.Fatalf("bad channels: %#v", channels)
	}
	posts, err := client.LoadPosts(context.Background(), "c1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 || posts[0].ID != "p1" || posts[0].ReplyCount != 1 || !posts[0].ThreadUnread || posts[1].ID != "p2" || posts[1].Unread {
		t.Fatalf("bad posts order/reply count: %#v", posts)
	}
	if len(posts[1].Reactions) != 1 || posts[1].Reactions[0].Name != "+1" || !posts[1].Reactions[0].Reacted || posts[1].Reactions[0].Count != 1 {
		t.Fatalf("bad post reactions: %#v", posts[1].Reactions)
	}
	if len(posts[1].Files) != 1 || posts[1].Files[0].Name != "photo.png" || posts[1].Files[0].MIMEType != "image/png" || posts[1].Files[0].Width != 640 {
		t.Fatalf("bad post files: %#v", posts[1].Files)
	}
	if posts[0].Username != "Sasha Neupokoev" || posts[1].Username != "Alice Smith" {
		t.Fatalf("bad usernames: %#v", posts)
	}
	older, err := client.LoadPostsBefore(context.Background(), "c1", "p1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(older) != 1 || older[0].ID != "p0" || older[0].Username != "Alice Smith" {
		t.Fatalf("bad older posts: %#v", older)
	}
	thread, err := client.LoadThread(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(thread) != 3 || thread[0].ID != "p1" || thread[1].ID != "r1" || thread[2].ID != "r2" || thread[1].Username != "Alice Smith" || thread[2].Unread != true {
		t.Fatalf("bad thread: %#v", thread)
	}
	if len(thread[1].Reactions) != 1 || thread[1].Reactions[0].Name != "heart" || !thread[1].Reactions[0].Reacted || thread[1].Reactions[0].Count != 1 {
		t.Fatalf("bad thread reactions: %#v", thread[1].Reactions)
	}
	if err := client.ViewChannel(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	post, err := client.SendPost(context.Background(), "c1", "sent")
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != "p3" || post.Username != "Sasha Neupokoev" {
		t.Fatalf("bad sent post: %#v", post)
	}
}

func TestUpdatePost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/posts/p1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["message"] != "edited" {
			t.Fatalf("message = %q", body["message"])
		}
		_ = json.NewEncoder(w).Encode(mmPost{ID: "p1", ChannelID: "c1", UserID: "u1", Message: "edited", CreateAt: 1, UpdateAt: 2})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(config.Config{ServerURL: server.URL, Token: "token"})
	client.cacheUsers([]mmUser{{ID: "u1", FirstName: "Sasha", LastName: "Neupokoev"}})
	post, err := client.UpdatePost(context.Background(), "p1", "edited")
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != "p1" || post.ChannelID != "c1" || post.Message != "edited" || post.UserID != "u1" || post.UpdateAt != 2 || post.Username != "Sasha Neupokoev" {
		t.Fatalf("bad updated post: %#v", post)
	}
}

func TestDeletePost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/posts/p1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(config.Config{ServerURL: server.URL, Token: "token"})
	if err := client.DeletePost(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
}

func TestAddReaction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/reactions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["user_id"] != "u1" || body["post_id"] != "p1" || body["emoji_name"] != "+1" {
			t.Fatalf("unexpected reaction body: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(mmReaction{UserID: "u1", PostID: "p1", EmojiName: "+1"})
	})
	mux.HandleFunc("/api/v4/posts/p1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("get method = %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(mmPost{
			ID:        "p1",
			ChannelID: "c1",
			UserID:    "u2",
			Message:   "hello",
			Metadata: &mmPostMetadata{Reactions: []mmReaction{
				{UserID: "u1", PostID: "p1", EmojiName: "+1"},
				{UserID: "u2", PostID: "p1", EmojiName: "+1"},
				{UserID: "u2", PostID: "p1", EmojiName: "eyes"},
			}},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(config.Config{ServerURL: server.URL, Token: "token"})
	client.userID = "u1"
	post, err := client.AddReaction(context.Background(), "p1", "+1")
	if err != nil {
		t.Fatal(err)
	}
	if len(post.Reactions) != 2 {
		t.Fatalf("reactions = %#v", post.Reactions)
	}
	if post.Reactions[0].Name != "+1" || post.Reactions[0].Count != 2 || !post.Reactions[0].Reacted {
		t.Fatalf("bad own reaction aggregation: %#v", post.Reactions)
	}
	if post.Reactions[1].Name != "eyes" || post.Reactions[1].Count != 1 || post.Reactions[1].Reacted {
		t.Fatalf("bad other reaction aggregation: %#v", post.Reactions)
	}
}

func TestRemoveReaction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/users/u1/posts/p1/reactions/+1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v4/posts/p1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("get method = %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(mmPost{
			ID:        "p1",
			ChannelID: "c1",
			UserID:    "u2",
			Message:   "hello",
			Metadata: &mmPostMetadata{Reactions: []mmReaction{
				{UserID: "u2", PostID: "p1", EmojiName: "+1"},
			}},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := New(config.Config{ServerURL: server.URL, Token: "token"})
	client.userID = "u1"
	post, err := client.RemoveReaction(context.Background(), "p1", "+1")
	if err != nil {
		t.Fatal(err)
	}
	if len(post.Reactions) != 1 || post.Reactions[0].Name != "+1" || post.Reactions[0].Count != 1 || post.Reactions[0].Reacted {
		t.Fatalf("bad reaction state after remove: %#v", post.Reactions)
	}
}

func TestPostFilesToDomainFallsBackToFileIDs(t *testing.T) {
	files := postFilesToDomain(&mmPostMetadata{Files: []mmFileInfo{{ID: "f1", Name: "one.txt"}}}, []string{"f1", "f2"})
	if len(files) != 2 || files[0].ID != "f1" || files[0].Name != "one.txt" || files[1].ID != "f2" || files[1].Name != "" {
		t.Fatalf("files = %#v", files)
	}
}
