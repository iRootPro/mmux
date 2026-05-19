package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"band-tui/internal/config"
	"band-tui/internal/domain"
)

type Client struct {
	baseURL    string
	token      string
	username   string
	password   string
	httpClient *http.Client

	mu               sync.RWMutex
	userID           string
	displayNameCache map[string]string
	lastViewedAt     map[string]int64
	closed           bool
}

func New(cfg config.Config) *Client {
	return &Client{
		baseURL:          strings.TrimRight(cfg.ServerURL, "/"),
		token:            cfg.Token,
		username:         cfg.Username,
		password:         cfg.Password,
		displayNameCache: map[string]string{},
		lastViewedAt:     map[string]int64{},
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) Connect(ctx context.Context) (*domain.Session, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("server URL is empty")
	}
	if err := c.ensureSafeURL(); err != nil {
		return nil, err
	}
	if c.token == "" {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}

	var user mmUser
	if err := c.do(ctx, http.MethodGet, "/api/v4/users/me", nil, &user); err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	c.mu.Lock()
	c.userID = user.ID
	c.displayNameCache[user.ID] = user.displayName()
	c.mu.Unlock()

	var teams []mmTeam
	if err := c.do(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(user.ID)+"/teams", nil, &teams); err != nil {
		return nil, fmt.Errorf("get teams: %w", err)
	}

	out := &domain.Session{
		ServerURL: c.baseURL,
		User: domain.User{
			ID:          user.ID,
			Username:    user.Username,
			Nickname:    user.Nickname,
			FirstName:   user.FirstName,
			LastName:    user.LastName,
			DisplayName: user.displayName(),
			Email:       user.Email,
		},
		Teams: make([]domain.Team, 0, len(teams)),
	}
	for _, t := range teams {
		out.Teams = append(out.Teams, domain.Team{ID: t.ID, Name: t.Name, DisplayName: t.DisplayName})
	}
	out.Emojis = c.loadCustomEmojis(ctx)
	return out, nil
}

func (c *Client) LoadChannels(ctx context.Context, teamID string) ([]domain.Channel, error) {
	userID := c.currentUserID()
	if userID == "" {
		return nil, fmt.Errorf("not connected")
	}
	if teamID == "" {
		return nil, fmt.Errorf("team ID is empty")
	}

	var teamChannels []mmChannel
	path := "/api/v4/users/" + url.PathEscape(userID) + "/teams/" + url.PathEscape(teamID) + "/channels"
	if err := c.do(ctx, http.MethodGet, path, nil, &teamChannels); err != nil {
		return nil, fmt.Errorf("get channels: %w", err)
	}

	members := c.loadChannelMembers(ctx, userID, teamID)
	channels := append([]mmChannel(nil), teamChannels...)
	var allChannels []mmChannel
	if err := c.do(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(userID)+"/channels", nil, &allChannels); err == nil {
		seen := make(map[string]struct{}, len(channels))
		for _, ch := range channels {
			seen[ch.ID] = struct{}{}
		}
		for _, ch := range allChannels {
			if _, ok := seen[ch.ID]; ok {
				continue
			}
			if ch.Type == "D" || ch.Type == "G" || ch.TeamID == teamID {
				channels = append(channels, ch)
				seen[ch.ID] = struct{}{}
			}
		}
	}

	out := make([]domain.Channel, 0, len(channels))
	statusUserIDs := make([]string, 0)
	for _, ch := range channels {
		name := ch.DisplayName
		var channelUserIDs []string
		memberCount := ch.MemberCount
		if ch.Type == "D" || ch.Type == "G" {
			info := c.directChannelInfo(ctx, ch.ID, userID)
			if info.DisplayName != "" {
				name = info.DisplayName
			}
			channelUserIDs = info.UserIDs
			memberCount = len(info.UserIDs)
			statusUserIDs = append(statusUserIDs, info.UserIDs...)
		}
		if name == "" {
			name = ch.Name
		}
		member := members[ch.ID]
		unread := ch.TotalMsgCount - member.MsgCount
		if unread < 0 {
			unread = 0
		}
		out = append(out, domain.Channel{
			ID:           ch.ID,
			TeamID:       ch.TeamID,
			Name:         ch.Name,
			DisplayName:  name,
			Type:         ch.Type,
			Unread:       unread,
			Mentions:     member.MentionCount,
			LastPostAt:   ch.LastPostAt,
			LastViewedAt: member.LastViewedAt,
			Header:       ch.Header,
			Purpose:      ch.Purpose,
			MemberCount:  memberCount,
			UserIDs:      channelUserIDs,
		})
		c.mu.Lock()
		if c.lastViewedAt == nil {
			c.lastViewedAt = map[string]int64{}
		}
		c.lastViewedAt[ch.ID] = member.LastViewedAt
		c.mu.Unlock()
	}
	statuses := c.loadUserStatuses(ctx, statusUserIDs)
	for i := range out {
		out[i].Status = combinedStatus(out[i].UserIDs, statuses)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type == "D" && out[j].Type == "D" {
			if out[i].LastPostAt != out[j].LastPostAt {
				return out[i].LastPostAt > out[j].LastPostAt
			}
		}
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out, nil
}

func (c *Client) LoadPosts(ctx context.Context, channelID string, limit int) ([]domain.Post, error) {
	return c.loadPosts(ctx, channelID, "", limit)
}

func (c *Client) LoadPostsBefore(ctx context.Context, channelID, beforePostID string, limit int) ([]domain.Post, error) {
	if beforePostID == "" {
		return nil, fmt.Errorf("before post ID is empty")
	}
	return c.loadPosts(ctx, channelID, beforePostID, limit)
}

func (c *Client) channelLastViewedAt(channelID string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastViewedAt[channelID]
}

func (c *Client) loadPosts(ctx context.Context, channelID, beforePostID string, limit int) ([]domain.Post, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channel ID is empty")
	}
	if limit <= 0 {
		limit = 50
	}

	var list mmPostList
	path := fmt.Sprintf("/api/v4/channels/%s/posts?page=0&per_page=%d&include_file_metadata=true", url.PathEscape(channelID), limit)
	if beforePostID != "" {
		path += "&before=" + url.QueryEscape(beforePostID)
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, fmt.Errorf("get posts: %w", err)
	}

	lastViewedAt := c.channelLastViewedAt(channelID)
	posts := make([]domain.Post, 0, len(list.Order))
	replyCounts := map[string]int{}
	threadUnread := map[string]bool{}
	// Mattermost channel history can include thread replies. In Band's default
	// view those replies live inside the thread, so keep the main timeline to
	// root posts and use replies only to improve the visible reply counter.
	for i := len(list.Order) - 1; i >= 0; i-- {
		id := list.Order[i]
		p, ok := list.Posts[id]
		if !ok || p.DeleteAt != 0 {
			continue
		}
		if p.RootID != "" {
			replyCounts[p.RootID]++
			if lastViewedAt > 0 && p.CreateAt > lastViewedAt {
				threadUnread[p.RootID] = true
			}
			continue
		}
		post := c.postToDomain(p, channelID)
		if lastViewedAt > 0 && post.CreateAt > lastViewedAt {
			post.Unread = true
		}
		posts = append(posts, post)
	}
	for i := range posts {
		if n := replyCounts[posts[i].ID]; n > posts[i].ReplyCount {
			posts[i].ReplyCount = n
		}
		if threadUnread[posts[i].ID] {
			posts[i].ThreadUnread = true
		}
	}
	c.hydratePosts(ctx, posts)
	return posts, nil
}

func (c *Client) ViewChannel(ctx context.Context, channelID string) error {
	userID := c.currentUserID()
	if userID == "" {
		return fmt.Errorf("not connected")
	}
	if channelID == "" {
		return fmt.Errorf("channel ID is empty")
	}
	path := "/api/v4/channels/members/" + url.PathEscape(userID) + "/view"
	body := map[string]string{"channel_id": channelID, "prev_channel_id": ""}
	if err := c.do(ctx, http.MethodPost, path, body, nil); err != nil {
		return fmt.Errorf("view channel: %w", err)
	}
	c.mu.Lock()
	if c.lastViewedAt == nil {
		c.lastViewedAt = map[string]int64{}
	}
	c.lastViewedAt[channelID] = time.Now().UnixMilli()
	c.mu.Unlock()
	return nil
}

func (c *Client) LoadThread(ctx context.Context, postID string) ([]domain.Post, error) {
	if postID == "" {
		return nil, fmt.Errorf("post ID is empty")
	}
	var list mmPostList
	path := "/api/v4/posts/" + url.PathEscape(postID) + "/thread?include_file_metadata=true"
	if err := c.do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}
	posts := make([]domain.Post, 0, len(list.Order))
	for _, id := range list.Order {
		if p, ok := list.Posts[id]; ok && p.DeleteAt == 0 {
			post := c.postToDomain(p, p.ChannelID)
			if lastViewedAt := c.channelLastViewedAt(post.ChannelID); lastViewedAt > 0 && post.CreateAt > lastViewedAt {
				post.Unread = true
			}
			posts = append(posts, post)
		}
	}
	sort.SliceStable(posts, func(i, j int) bool {
		if posts[i].ID == postID {
			return true
		}
		if posts[j].ID == postID {
			return false
		}
		return posts[i].CreateAt < posts[j].CreateAt
	})
	c.hydratePosts(ctx, posts)
	return posts, nil
}

func (c *Client) SendPost(ctx context.Context, channelID, message string) (domain.Post, error) {
	return c.sendPost(ctx, channelID, "", message)
}

func (c *Client) SendReply(ctx context.Context, channelID, rootID, message string) (domain.Post, error) {
	if rootID == "" {
		return domain.Post{}, fmt.Errorf("root ID is empty")
	}
	return c.sendPost(ctx, channelID, rootID, message)
}

func (c *Client) UpdatePost(ctx context.Context, postID, message string) (domain.Post, error) {
	var post mmPost
	path := "/api/v4/posts/" + url.PathEscape(postID)
	body := map[string]string{"message": message}
	if err := c.do(ctx, http.MethodPut, path, body, &post); err != nil {
		return domain.Post{}, fmt.Errorf("update post: %w", err)
	}
	out := c.postToDomain(post, "")
	return out, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string) error {
	path := "/api/v4/posts/" + url.PathEscape(postID)
	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	return nil
}

func (c *Client) AddReaction(ctx context.Context, postID, emojiName string) (domain.Post, error) {
	userID := c.currentUserID()
	if userID == "" {
		return domain.Post{}, fmt.Errorf("not connected")
	}
	body := map[string]string{
		"user_id":    userID,
		"post_id":    postID,
		"emoji_name": emojiName,
	}
	if err := c.do(ctx, http.MethodPost, "/api/v4/reactions", body, nil); err != nil {
		return domain.Post{}, fmt.Errorf("add reaction: %w", err)
	}
	return c.loadPostByID(ctx, postID)
}

func (c *Client) RemoveReaction(ctx context.Context, postID, emojiName string) (domain.Post, error) {
	userID := c.currentUserID()
	if userID == "" {
		return domain.Post{}, fmt.Errorf("not connected")
	}
	path := "/api/v4/users/" + url.PathEscape(userID) + "/posts/" + url.PathEscape(postID) + "/reactions/" + url.PathEscape(emojiName)
	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return domain.Post{}, fmt.Errorf("remove reaction: %w", err)
	}
	return c.loadPostByID(ctx, postID)
}

func (c *Client) sendPost(ctx context.Context, channelID, rootID, message string) (domain.Post, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return domain.Post{}, fmt.Errorf("message is empty")
	}
	var post mmPost
	body := map[string]string{"channel_id": channelID, "message": message}
	if rootID != "" {
		body["root_id"] = rootID
	}
	if err := c.do(ctx, http.MethodPost, "/api/v4/posts", body, &post); err != nil {
		return domain.Post{}, fmt.Errorf("send post: %w", err)
	}
	out := c.postToDomain(post, channelID)
	return out, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *Client) currentUserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

func (c *Client) currentToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *Client) usernameFor(userID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.displayNameCache[userID]
}

func (c *Client) loadPostByID(ctx context.Context, postID string) (domain.Post, error) {
	var post mmPost
	if err := c.do(ctx, http.MethodGet, "/api/v4/posts/"+url.PathEscape(postID)+"?include_file_metadata=true", nil, &post); err != nil {
		return domain.Post{}, fmt.Errorf("get post: %w", err)
	}
	out := c.postToDomain(post, post.ChannelID)
	return out, nil
}

func (c *Client) postToDomain(post mmPost, fallbackChannelID string) domain.Post {
	out := post.toDomain(fallbackChannelID)
	out.Username = c.usernameFor(out.UserID)
	out.Reactions = c.reactionsToDomain(post.Metadata)
	return out
}

func (c *Client) reactionsToDomain(metadata *mmPostMetadata) []domain.PostReaction {
	if metadata == nil || len(metadata.Reactions) == 0 {
		return nil
	}
	currentUserID := c.currentUserID()
	indexByName := make(map[string]int, len(metadata.Reactions))
	out := make([]domain.PostReaction, 0, len(metadata.Reactions))
	for _, reaction := range metadata.Reactions {
		idx, ok := indexByName[reaction.EmojiName]
		if !ok {
			idx = len(out)
			indexByName[reaction.EmojiName] = idx
			out = append(out, domain.PostReaction{Name: reaction.EmojiName})
		}
		out[idx].Count++
		if currentUserID != "" && reaction.UserID == currentUserID {
			out[idx].Reacted = true
		}
	}
	return out
}
func (c *Client) cacheUsers(users []mmUser) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.displayNameCache == nil {
		c.displayNameCache = map[string]string{}
	}
	for _, user := range users {
		if user.ID != "" {
			c.displayNameCache[user.ID] = user.displayName()
		}
	}
}

func (c *Client) loadChannelMembers(ctx context.Context, userID, teamID string) map[string]mmChannelMember {
	out := map[string]mmChannelMember{}
	var teamMembers []mmChannelMember
	path := "/api/v4/users/" + url.PathEscape(userID) + "/teams/" + url.PathEscape(teamID) + "/channels/members"
	if err := c.do(ctx, http.MethodGet, path, nil, &teamMembers); err == nil {
		for _, member := range teamMembers {
			out[member.ChannelID] = member
		}
	}
	var allMembers []mmChannelMember
	path = "/api/v4/users/" + url.PathEscape(userID) + "/channels/members"
	if err := c.do(ctx, http.MethodGet, path, nil, &allMembers); err == nil {
		for _, member := range allMembers {
			out[member.ChannelID] = member
		}
	}
	return out
}

type directChannelInfo struct {
	DisplayName string
	UserIDs     []string
}

func (c *Client) directChannelInfo(ctx context.Context, channelID, currentUserID string) directChannelInfo {
	var users []mmUser
	path := "/api/v4/users?in_channel=" + url.QueryEscape(channelID) + "&page=0&per_page=20"
	if err := c.do(ctx, http.MethodGet, path, nil, &users); err != nil {
		return directChannelInfo{}
	}
	c.cacheUsers(users)
	names := make([]string, 0, len(users))
	ids := make([]string, 0, len(users))
	for _, user := range users {
		if user.ID == "" || user.ID == currentUserID {
			continue
		}
		ids = append(ids, user.ID)
		if name := user.displayName(); name != "" {
			names = append(names, name)
		}
	}
	return directChannelInfo{DisplayName: strings.Join(names, ", "), UserIDs: ids}
}

func (c *Client) loadCustomEmojis(ctx context.Context) []domain.Emoji {
	const pageSize = 200
	out := make([]domain.Emoji, 0)
	seen := map[string]struct{}{}
	for page := 0; page < 20; page++ {
		var emojis []mmEmoji
		path := fmt.Sprintf("/api/v4/emoji?page=%d&per_page=%d", page, pageSize)
		if err := c.do(ctx, http.MethodGet, path, nil, &emojis); err != nil {
			break
		}
		for _, emoji := range emojis {
			name := strings.TrimSpace(emoji.Name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, domain.Emoji{Name: name})
		}
		if len(emojis) < pageSize {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (c *Client) loadUserStatuses(ctx context.Context, userIDs []string) map[string]string {
	unique := make([]string, 0, len(userIDs))
	seen := map[string]struct{}{}
	for _, id := range userIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil
	}
	var statuses []mmStatus
	if err := c.do(ctx, http.MethodPost, "/api/v4/users/status/ids", unique, &statuses); err != nil {
		return nil
	}
	out := make(map[string]string, len(statuses))
	for _, st := range statuses {
		if st.UserID != "" {
			out[st.UserID] = st.Status
		}
	}
	return out
}

func combinedStatus(userIDs []string, statuses map[string]string) string {
	if len(userIDs) == 0 || len(statuses) == 0 {
		return ""
	}
	best := "offline"
	for _, id := range userIDs {
		switch statuses[id] {
		case "online":
			return "online"
		case "away":
			if best == "offline" {
				best = "away"
			}
		case "dnd":
			if best != "away" {
				best = "dnd"
			}
		}
	}
	return best
}

func (c *Client) hydratePosts(ctx context.Context, posts []domain.Post) {
	missing := make([]string, 0)
	seen := map[string]struct{}{}
	for _, post := range posts {
		if post.UserID == "" || c.usernameFor(post.UserID) != "" {
			continue
		}
		if _, ok := seen[post.UserID]; ok {
			continue
		}
		seen[post.UserID] = struct{}{}
		missing = append(missing, post.UserID)
	}
	if len(missing) > 0 {
		var users []mmUser
		if err := c.do(ctx, http.MethodPost, "/api/v4/users/ids", missing, &users); err == nil {
			c.cacheUsers(users)
		}
	}
	for i := range posts {
		posts[i].Username = c.usernameFor(posts[i].UserID)
	}
}

func (c *Client) ensureSafeURL() error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLocalhost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("refusing to send credentials to insecure server URL %q; use https", c.baseURL)
}

func isLocalhost(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (c *Client) login(ctx context.Context) error {
	if c.username == "" || c.password == "" {
		return fmt.Errorf("no token and no username/password; set BAND_TOKEN or BAND_USERNAME/BAND_PASSWORD")
	}
	body := map[string]string{"login_id": c.username, "password": c.password}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v4/users/login", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return wrapRequestError("login request", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return wrapHTTPError("login", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	token := resp.Header.Get("Token")
	if token == "" {
		token = resp.Header.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")
	}
	if token == "" {
		return fmt.Errorf("login succeeded but server did not return a token header")
	}
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
	return nil
}

func wrapHTTPError(op string, status int, message string) error {
	kind := domain.BackendErrorUnknown
	retryable := false
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		kind = domain.BackendErrorAuth
		retryable = false
	case status >= http.StatusInternalServerError:
		kind = domain.BackendErrorServer
		retryable = true
	}
	var err error
	if message != "" {
		err = errors.New(message)
	}
	return &domain.BackendError{
		Op:         op,
		Kind:       kind,
		StatusCode: status,
		Retryable:  retryable,
		Err:        err,
	}
}

func wrapRequestError(op string, err error) error {
	kind := domain.BackendErrorUnknown
	retryable := false
	var netErr net.Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		kind = domain.BackendErrorNetwork
		retryable = false
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr):
		kind = domain.BackendErrorNetwork
		retryable = true
	}
	return &domain.BackendError{
		Op:        op,
		Kind:      kind,
		Retryable: retryable,
		Err:       err,
	}
}

func (c *Client) do(ctx context.Context, method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return wrapRequestError(method+" "+path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return wrapHTTPError(method+" "+path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type mmUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

func (u mmUser) displayName() string {
	fullName := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if fullName != "" {
		return fullName
	}
	if strings.TrimSpace(u.Nickname) != "" {
		return strings.TrimSpace(u.Nickname)
	}
	return u.Username
}

type mmTeam struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type mmChannel struct {
	ID            string `json:"id"`
	TeamID        string `json:"team_id"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Type          string `json:"type"`
	Header        string `json:"header"`
	Purpose       string `json:"purpose"`
	MemberCount   int    `json:"member_count"`
	TotalMsgCount int    `json:"total_msg_count"`
	LastPostAt    int64  `json:"last_post_at"`
}

type mmChannelMember struct {
	ChannelID    string `json:"channel_id"`
	MsgCount     int    `json:"msg_count"`
	MentionCount int    `json:"mention_count"`
	LastViewedAt int64  `json:"last_viewed_at"`
}

type mmStatus struct {
	UserID string `json:"user_id"`
	Status string `json:"status"`
}

type mmEmoji struct {
	Name string `json:"name"`
}

type mmPostList struct {
	Order []string          `json:"order"`
	Posts map[string]mmPost `json:"posts"`
}

type mmReaction struct {
	UserID    string `json:"user_id"`
	PostID    string `json:"post_id"`
	EmojiName string `json:"emoji_name"`
}

type mmPostMetadata struct {
	Reactions []mmReaction `json:"reactions,omitempty"`
	Files     []mmFileInfo `json:"files,omitempty"`
}

type mmFileInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
	MIMEType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type mmPost struct {
	ID         string          `json:"id"`
	ChannelID  string          `json:"channel_id"`
	RootID     string          `json:"root_id"`
	UserID     string          `json:"user_id"`
	Message    string          `json:"message"`
	FileIDs    []string        `json:"file_ids,omitempty"`
	CreateAt   int64           `json:"create_at"`
	UpdateAt   int64           `json:"update_at"`
	DeleteAt   int64           `json:"delete_at"`
	ReplyCount int             `json:"reply_count"`
	Metadata   *mmPostMetadata `json:"metadata,omitempty"`
}

func (p mmPost) toDomain(fallbackChannelID string) domain.Post {
	ch := p.ChannelID
	if ch == "" {
		ch = fallbackChannelID
	}
	return domain.Post{
		ID:         p.ID,
		ChannelID:  ch,
		RootID:     p.RootID,
		UserID:     p.UserID,
		Message:    p.Message,
		CreateAt:   p.CreateAt,
		UpdateAt:   p.UpdateAt,
		ReplyCount: p.ReplyCount,
		Files:      postFilesToDomain(p.Metadata, p.FileIDs),
	}
}

func postFilesToDomain(metadata *mmPostMetadata, fileIDs []string) []domain.PostFile {
	out := make([]domain.PostFile, 0, len(fileIDs))
	seen := make(map[string]struct{}, len(fileIDs))
	if metadata != nil {
		out = make([]domain.PostFile, 0, max(len(metadata.Files), len(fileIDs)))
		for _, file := range metadata.Files {
			if file.ID == "" && file.Name == "" {
				continue
			}
			out = append(out, domain.PostFile{
				ID:        file.ID,
				Name:      file.Name,
				Extension: file.Extension,
				MIMEType:  file.MIMEType,
				Size:      file.Size,
				Width:     file.Width,
				Height:    file.Height,
			})
			if file.ID != "" {
				seen[file.ID] = struct{}{}
			}
		}
	}
	for _, id := range fileIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		out = append(out, domain.PostFile{ID: id})
	}
	return out
}
