package gwcli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/textproto"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	drive "google.golang.org/api/drive/v3"
	gmail "google.golang.org/api/gmail/v1"
	people "google.golang.org/api/people/v1"
	"google.golang.org/api/tasks/v1"
)

const (
	// Scope for Gmail API with gmailctl compatibility.
	scope = "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/gmail.settings.basic https://www.googleapis.com/auth/gmail.labels"

	pageSize = 100

	accessType = "offline"
	email      = "me"
)

// Different levels of detail to download.
const (
	LevelEmpty    DataLevel = ""         // Nothing
	LevelMinimal  DataLevel = "minimal"  // ID, labels
	LevelMetadata DataLevel = "metadata" // ID, labels, headers
	LevelFull     DataLevel = "full"     // ID, labels, headers, payload
)

const (
	// Not so much a level as a separate request. Type `string` so that it won't be usable as a `DataLevel`.
	levelRaw string = "RAW"

	appDataFolder = "appDataFolder"

	settingsFileName = "settings.json"
)

var (
	// Version is the app version as reported in RPCs.
	Version = "unspecified"

	// NewThread is the thread ID to use for new threads.
	NewThread ThreadID

	shouldLogRPC = flag.Bool("log_rpc", false, "Log all RPCs.")
	socks5       = flag.String("socks5", "", "Use SOCKS5 proxy. host:port")
)

type (
	// DataLevel is the level of detail to download.
	DataLevel string

	// HistoryID is a sparse timeline of numbers.
	HistoryID uint64

	// ThreadID is IDs of threads.
	ThreadID string
)

// CmdG is the main app for cmdg. It holds both rpc clients and various caches. Everything except the UI.
type CmdG struct {
	m            sync.RWMutex
	authedClient *http.Client
	gmail        *gmail.Service
	drive        *drive.Service
	people       *people.Service
	tasks        *tasks.Service
	calendar     *calendar.Service
	messageCache map[string]*Message
	labelCache   map[string]*Label
	labelsLoaded bool // tracks whether labels have been loaded from config
	contacts     []string
	settings     Settings
	configPaths  *ConfigPaths
}

func userAgent() string {
	return "cmdg " + Version
}

// MessageCache returns the message from the cache, or nil if not found.
func (c *CmdG) MessageCache(msg *Message) *Message {
	c.m.Lock()
	defer c.m.Unlock()
	if t, found := c.messageCache[msg.ID]; found {
		return t
	}
	c.messageCache[msg.ID] = msg
	return msg
}

// LabelCache returns the label fro the cache, or nil if not found.
func (c *CmdG) LabelCache(label *Label) *Label {
	c.m.Lock()
	defer c.m.Unlock()
	if t, f := c.labelCache[label.ID]; f {
		return t
	}
	c.labelCache[label.ID] = label
	return label
}

// NewFake creates a fake client, used for testing.
func NewFake(client *http.Client) (*CmdG, error) {
	conn := &CmdG{
		authedClient: client,
	}
	return conn, conn.setupClients()
}

func readConf(fn string) (Config, error) {
	f, err := ioutil.ReadFile(fn)
	if err != nil {
		return Config{}, err
	}
	var conf Config
	if err := json.Unmarshal(f, &conf); err != nil {
		return Config{}, errors.Wrapf(err, "unmarshalling config")
	}
	if conf.OAuth.ClientID == "" {
		conf.OAuth.ClientID = DefaultClientID
		conf.OAuth.ClientSecret = DefaultClientSecret
	}
	return conf, nil
}

// New creates a new CmdG with gmailctl-style authentication.
// configDir should point to the directory containing credentials.json and token.json
// userEmail is only required when using service account authentication (for user impersonation)
// verbose enables detailed logging of the connection setup process
func New(configDir string, userEmail string, verbose bool) (*CmdG, error) {
	conn := &CmdG{
		messageCache: make(map[string]*Message),
		labelCache:   make(map[string]*Label),
	}

	// Get config paths
	paths, err := GetConfigPaths(configDir)
	if err != nil {
		return nil, err
	}

	if verbose {
		log.Infof("Config paths resolved:")
		log.Infof("  Directory: %s", paths.Dir)
		log.Infof("  Credentials: %s", paths.Credentials)
		log.Infof("  Token: %s", paths.Token)
		log.Infof("  Config: %s", paths.Config)
	}

	// Store config paths for later use (e.g., loading labels from config.jsonnet)
	conn.configPaths = paths

	// Initialize Gmail service using gmailctl authentication
	ctx := context.Background()
	if verbose {
		log.Infof("Initializing Gmail authentication...")
	}
	gmailSvc, err := InitializeAuth(ctx, paths, userEmail)
	if err != nil {
		return nil, err
	}

	// Store the gmail service
	conn.gmail = gmailSvc

	// Check if using service account (no token file needed)
	credFile, err := os.Open(paths.Credentials)
	if err != nil {
		return nil, errors.Wrapf(err, "opening credentials")
	}
	defer credFile.Close()

	isServiceAcct, err := gmailctl.IsServiceAccount(credFile)
	if err != nil {
		return nil, errors.Wrapf(err, "detecting credential type")
	}

	if verbose {
		if isServiceAcct {
			log.Infof("Authentication type: Service Account")
		} else {
			log.Infof("Authentication type: OAuth2")
		}
	}

	// Service accounts don't need Drive/People client setup (no token file)
	if isServiceAcct {
		// For service accounts, we can't set up Drive and People APIs the same way
		// Just set up the Gmail client which we already have
		conn.authedClient = nil // Service accounts don't use the same auth flow

		// Initialize Tasks service for service accounts
		credFile2, err := os.Open(paths.Credentials)
		if err != nil {
			return nil, errors.Wrapf(err, "opening credentials for tasks service")
		}
		defer credFile2.Close()

		serviceAcctAuth, err := gmailctl.NewServiceAccountAuthenticator(credFile2, userEmail)
		if err != nil {
			return nil, errors.Wrapf(err, "creating service account authenticator for tasks")
		}

		tasksSvc, err := serviceAcctAuth.TasksService(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "creating tasks service for service account")
		}
		conn.tasks = tasksSvc

		// Initialize Calendar service for service accounts
		credFile3, err := os.Open(paths.Credentials)
		if err != nil {
			return nil, errors.Wrapf(err, "opening credentials for calendar service")
		}
		defer credFile3.Close()

		serviceAcctAuth2, err := gmailctl.NewServiceAccountAuthenticator(credFile3, userEmail)
		if err != nil {
			return nil, errors.Wrapf(err, "creating service account authenticator for calendar")
		}

		calSvc, err := serviceAcctAuth2.CalendarService(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "creating calendar service for service account")
		}
		conn.calendar = calSvc

		if verbose {
			log.Infof("Service account connection ready")
		}
		return conn, nil
	}

	// For OAuth: Get authenticated HTTP client for Drive and People APIs
	tokenFile, err := os.Open(paths.Token)
	if err != nil {
		return nil, errors.Wrapf(err, "opening token")
	}
	defer tokenFile.Close()

	tokBytes, err := ioutil.ReadAll(tokenFile)
	if err != nil {
		return nil, errors.Wrapf(err, "reading token")
	}

	var tok oauth2.Token
	if err := json.Unmarshal(tokBytes, &tok); err != nil {
		return nil, errors.Wrapf(err, "parsing token")
	}

	// Get OAuth config to create HTTP client
	cfg, err := getOAuthConfig(paths.Credentials)
	if err != nil {
		return nil, err
	}

	conn.authedClient = cfg.Client(ctx, &tok)

	if verbose {
		log.Infof("OAuth connection ready")
	}

	// Set up Drive and People services (note: gwcli doesn't actively use these,
	// but keeping them for compatibility with old cmdg code)
	return conn, conn.setupClients()
}

// getOAuthConfig creates an OAuth2 config from credentials file
func getOAuthConfig(credPath string) (*oauth2.Config, error) {
	credFile, err := os.Open(credPath)
	if err != nil {
		return nil, errors.Wrapf(err, "opening credentials")
	}
	defer credFile.Close()

	credBytes, err := ioutil.ReadAll(credFile)
	if err != nil {
		return nil, errors.Wrapf(err, "reading credentials")
	}

	return oauth2Config(credBytes)
}

// oauth2Config creates OAuth2 config from credential bytes
func oauth2Config(credBytes []byte) (*oauth2.Config, error) {
	type creds struct {
		Installed struct {
			ClientID     string   `json:"client_id"`
			ClientSecret string   `json:"client_secret"`
			RedirectURIs []string `json:"redirect_uris"`
			AuthURI      string   `json:"auth_uri"`
			TokenURI     string   `json:"token_uri"`
		} `json:"installed"`
	}

	var c creds
	if err := json.Unmarshal(credBytes, &c); err != nil {
		return nil, errors.Wrapf(err, "parsing credentials")
	}

	return &oauth2.Config{
		ClientID:     c.Installed.ClientID,
		ClientSecret: c.Installed.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.Installed.AuthURI,
			TokenURL: c.Installed.TokenURI,
		},
		Scopes: []string{
			"https://www.googleapis.com/auth/gmail.modify",
			"https://www.googleapis.com/auth/gmail.settings.basic",
			"https://www.googleapis.com/auth/gmail.labels",
		},
	}, nil
}

func (c *CmdG) setupClients() error {
	// Set up gmail client.
	{
		var err error
		c.gmail, err = gmail.New(c.authedClient)
		if err != nil {
			return errors.Wrap(err, "creating GMail client")
		}
		c.gmail.UserAgent = userAgent()
	}

	// Set up drive client.
	{
		var err error
		c.drive, err = drive.New(c.authedClient)
		if err != nil {
			return errors.Wrap(err, "creating Drive client")
		}
		c.drive.UserAgent = userAgent()
	}
	// Set up people client.
	{
		var err error
		c.people, err = people.New(c.authedClient)
		if err != nil {
			return errors.Wrap(err, "creating People client")
		}
		c.people.UserAgent = userAgent()
	}
	// Set up tasks client.
	{
		var err error
		c.tasks, err = tasks.New(c.authedClient)
		if err != nil {
			return errors.Wrap(err, "creating Tasks client")
		}
		c.tasks.UserAgent = userAgent()
	}
	// Set up calendar client.
	{
		var err error
		c.calendar, err = calendar.New(c.authedClient)
		if err != nil {
			return errors.Wrap(err, "creating Calendar client")
		}
		c.calendar.UserAgent = userAgent()
	}
	return nil
}

func wrapLogRPC(fn string, cb func() error, af string, args ...interface{}) error {
	st := time.Now()
	err := cb()
	logRPC(st, err, fmt.Sprintf("%s(%s)", fn, af), args...)
	return err
}

func logRPC(st time.Time, err error, s string, args ...interface{}) {
	if *shouldLogRPC {
		log.Infof("RPC> %s => %v %v", fmt.Sprintf(s, args...), err, time.Since(st))
	}
}

// LoadLabels batch loads all labels into the cache from config.jsonnet.
func (c *CmdG) LoadLabels(ctx context.Context, verbose bool) error {
	// Check if already loaded (with read lock to avoid unnecessary work)
	c.m.RLock()
	if c.labelsLoaded {
		c.m.RUnlock()
		return nil
	}
	c.m.RUnlock()

	st := time.Now()

	// Read labels from config.jsonnet (required - no API fallback)
	configPath := c.configPaths.Config
	if verbose {
		log.Infof("Loading labels from config file: %s", configPath)
	}

	cfg, err := gmailctl.ReadFile(configPath, "")
	if err != nil {
		if err == gmailctl.ErrNotFound {
			return fmt.Errorf("config.jsonnet not found at %s: gmailctl integration requires this file. See https://github.com/mbrt/gmailctl for setup instructions", configPath)
		}
		return fmt.Errorf("failed to read config.jsonnet: %w", err)
	}

	if verbose {
		log.Infof("Successfully parsed config.jsonnet (version: %s)", cfg.Version)
	}

	c.m.Lock()
	defer c.m.Unlock()

	// Fetch all labels from Gmail API to get actual label IDs
	var gmailLabels []*gmail.Label
	err = wrapLogRPC("gmail.Users.Labels.List", func() (err error) {
		var resp *gmail.ListLabelsResponse
		resp, err = c.gmail.Users.Labels.List(email).Context(ctx).Do()
		if err == nil {
			gmailLabels = resp.Labels
		}
		return
	}, "email=%q", email)
	if err != nil {
		return fmt.Errorf("failed to fetch labels from Gmail API: %w", err)
	}

	// Build a map of label names to Gmail API label IDs
	gmailLabelsByName := make(map[string]*gmail.Label)
	for _, gl := range gmailLabels {
		gmailLabelsByName[gl.Name] = gl
	}

	// Add system labels first (Gmail built-ins that should always be available)
	systemLabels := map[string]string{
		"INBOX":               "INBOX",
		"TRASH":               "Trash",
		"UNREAD":              "UNREAD",
		"STARRED":             "Starred",
		"SENT":                "SENT",
		"DRAFT":               "DRAFT",
		"SPAM":                "SPAM",
		"IMPORTANT":           "IMPORTANT",
		"CATEGORY_PERSONAL":   "Personal",
		"CATEGORY_SOCIAL":     "Social",
		"CATEGORY_PROMOTIONS": "Promotions",
		"CATEGORY_UPDATES":    "Updates",
		"CATEGORY_FORUMS":     "Forums",
	}

	for id, name := range systemLabels {
		c.labelCache[id] = &Label{
			ID:    id,
			Label: name,
			Response: &gmail.Label{
				Id:   id,
				Name: name,
				Type: "system",
			},
		}
	}

	// Add labels from config.jsonnet, using actual Gmail API label IDs
	for _, l := range cfg.Labels {
		// Look up the actual label ID from Gmail API
		gmailLabel, found := gmailLabelsByName[l.Name]
		if !found {
			if verbose {
				log.Warnf("Label '%s' from config.jsonnet not found in Gmail - skipping", l.Name)
			}
			continue
		}

		// Use the actual Gmail API label ID
		c.labelCache[gmailLabel.Id] = &Label{
			ID:       gmailLabel.Id,
			Label:    gmailLabel.Name,
			Response: gmailLabel,
		}
	}

	if verbose {
		log.Infof("Loaded %d labels (%d from config.jsonnet, %d system labels) in %v",
			len(c.labelCache), len(cfg.Labels), len(systemLabels), time.Since(st))
	} else {
		log.Infof("Loaded %d labels in %v", len(c.labelCache), time.Since(st))
	}

	// Mark labels as loaded
	c.labelsLoaded = true

	return nil
}

// Labels returns a list of all labels.
// Labels are lazily loaded from config.jsonnet on first access.
func (c *CmdG) Labels() []*Label {
	// Check if labels need to be loaded
	c.m.RLock()
	loaded := c.labelsLoaded
	c.m.RUnlock()

	// Lazy load labels if not yet loaded
	if !loaded {
		// Use background context and non-verbose mode for lazy loading
		// Errors are silently ignored - empty label list is returned
		_ = c.LoadLabels(context.Background(), false)
	}

	c.m.RLock()
	defer c.m.RUnlock()
	var ret []*Label
	for _, l := range c.labelCache {
		ret = append(ret, l)
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].ID == Inbox {
			return true
		}
		if ret[j].ID == Inbox {
			return false
		}
		if ret[i].ID == Trash {
			return false
		}
		if ret[j].ID == Trash {
			return true
		}
		return ret[i].Label < ret[j].Label
	})
	return ret
}

// GmailService returns the Gmail API service.
func (c *CmdG) GmailService() *gmail.Service {
	return c.gmail
}

// TasksService returns the Google Tasks API service client.
func (c *CmdG) TasksService() *tasks.Service {
	return c.tasks
}

// CalendarService returns the Google Calendar API service client.
func (c *CmdG) CalendarService() *calendar.Service {
	return c.calendar
}

// GetProfile returns the profile for the current user.
func (c *CmdG) GetProfile(ctx context.Context) (*gmail.Profile, error) {
	var ret *gmail.Profile
	err := wrapLogRPC("gmail.Users.GetProfile", func() (err error) {
		ret, err = c.gmail.Users.GetProfile(email).Context(ctx).Do()
		return
	}, "email=%q", email)
	return ret, err
}

// TokenInfo contains information about the OAuth token.
type TokenInfo struct {
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	ExpiresIn     int64    `json:"expires_in"`
	Scope         string   `json:"scope"`
	Scopes        []string `json:"scopes"`
	UserID        string   `json:"user_id"`
	Audience      string   `json:"aud"`
	IssuedTo      string   `json:"issued_to"`
	AppName       string   `json:"app_name"`
}

// UnmarshalJSON allows TokenInfo to accept either numeric or quoted numeric expires_in values.
func (t *TokenInfo) UnmarshalJSON(data []byte) error {
	type tokenInfoAlias struct {
		Email         string          `json:"email"`
		EmailVerified bool            `json:"email_verified"`
		ExpiresIn     json.RawMessage `json:"expires_in"`
		Scope         string          `json:"scope"`
		Scopes        []string        `json:"scopes"`
		UserID        string          `json:"user_id"`
		Audience      string          `json:"aud"`
		IssuedTo      string          `json:"issued_to"`
		AppName       string          `json:"app_name"`
	}

	var aux tokenInfoAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	expiresIn, err := parseFlexibleInt64(aux.ExpiresIn)
	if err != nil {
		return errors.Wrap(err, "parsing expires_in")
	}

	t.Email = aux.Email
	t.EmailVerified = aux.EmailVerified
	t.ExpiresIn = expiresIn
	t.Scope = aux.Scope
	t.Scopes = aux.Scopes
	t.UserID = aux.UserID
	t.Audience = aux.Audience
	t.IssuedTo = aux.IssuedTo
	t.AppName = aux.AppName

	return nil
}

func parseFlexibleInt64(data json.RawMessage) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}

	var numeric int64
	if err := json.Unmarshal(data, &numeric); err == nil {
		return numeric, nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" {
			return 0, nil
		}
		v, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return 0, err
		}
		return v, nil
	}

	return 0, errors.Errorf("expires_in must be number or quoted number, got %s", string(data))
}

// GetTokenInfo retrieves information about the current OAuth token.
func (c *CmdG) GetTokenInfo(ctx context.Context) (*TokenInfo, error) {
	if c.authedClient == nil {
		return nil, errors.New("token info not available for service account authentication")
	}

	// Use the authenticated client to make a request to the tokeninfo endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", "https://oauth2.googleapis.com/tokeninfo", nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating tokeninfo request")
	}

	resp, err := c.authedClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "fetching token info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, errors.Errorf("tokeninfo request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenInfo TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, errors.Wrap(err, "decoding token info response")
	}

	// Parse the space-separated scopes into an array
	if tokenInfo.Scope != "" {
		tokenInfo.Scopes = strings.Split(tokenInfo.Scope, " ")
	}

	return &tokenInfo, nil
}

// Part is a part of a message. Contents and header.
type Part struct {
	Contents string
	Header   textproto.MIMEHeader
}

// FullString returns the "serialized" part.
func (p *Part) FullString() string {
	var hs []string
	for k, vs := range p.Header {
		for _, v := range vs {
			hs = append(hs, fmt.Sprintf("%s: %s", k, v))
		}
	}
	// TODO: this can't be right. Go libraries use maps for headers but we depend on order. ;-(
	sort.Slice(hs, func(i, j int) bool {
		if strings.HasPrefix(strings.ToLower(hs[i]), "content-type: ") {
			return true
		}
		return hs[i] < hs[j]
	})
	return strings.Join(hs, "\r\n") + "\r\n\r\n" + p.Contents
}

// ParseUserMessage parses what's in the user's editor and turns into into a Part and message headers.
func ParseUserMessage(in string) (mail.Header, *Part, error) {
	m, err := mail.ReadMessage(strings.NewReader(in))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "message to send is malformed")
	}
	b, err := ioutil.ReadAll(m.Body)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read user message")
	}
	m.Header["MIME-Version"] = []string{"1.0"}
	return m.Header, &Part{
		Header: map[string][]string{
			"Content-Type":        []string{`text/plain; charset="UTF-8"`},
			"Content-Disposition": []string{"inline"},
		},
		Contents: string(b),
	}, nil
}

// SendParts sends a multipart message.
// Args:
//
//	mp:    multipart type. "mixed" is a typical type.
//	head:  Email header.
//	parts: Email parts.
func (c *CmdG) SendParts(ctx context.Context, threadID ThreadID, mp string, head mail.Header, parts []*Part) error {
	var mbuf bytes.Buffer
	w := multipart.NewWriter(&mbuf)

	// Create mail contents.
	for _, p := range parts {
		p2, err := w.CreatePart(p.Header)
		if err != nil {
			return errors.Wrapf(err, "failed to create part")
		}
		if _, err := p2.Write([]byte(p.Contents)); err != nil {
			return errors.Wrapf(err, "assembling part")
		}
	}
	if err := w.Close(); err != nil {
		return errors.Wrapf(err, "closing multipart")
	}

	addrHeader := map[string]bool{
		"to":       true,
		"cc":       true,
		"bcc":      true,
		"reply-to": true,
	}

	// Add message headers for gmail.
	var hlines []string
	for k, vs := range head {
		if addrHeader[strings.ToLower(k)] {
			for _, v := range vs {
				if v == "" {
					continue
				}
				as, err := mail.ParseAddressList(v)
				if err != nil {
					return errors.Wrapf(err, "parsing address list %q, which is %q", k, v)
				}
				var ass []string
				for _, a := range as {
					if a.Name == "" {
						ass = append(ass, a.Address)
					} else {
						ass = append(ass, fmt.Sprintf(`"%s" <%s>`, mime.QEncoding.Encode("utf-8", a.Name), a.Address))
					}
				}
				hlines = append(hlines, fmt.Sprintf("%s: %s", k, strings.Join(ass, ", ")))
			}
		} else {
			for _, v := range vs {
				hlines = append(hlines, fmt.Sprintf("%s: %s", k, mime.QEncoding.Encode("utf-8", v)))

			}
		}
	}
	sort.Strings(hlines)
	hlines = append(hlines, fmt.Sprintf(`Content-Type: multipart/%s; boundary="%s"`, mp, w.Boundary()))
	hlines = append(hlines, `Content-Disposition: inline`)
	msgs := strings.Join(hlines, "\r\n") + "\r\n\r\n" + mbuf.String()

	log.Infof("Final message: %q", msgs)
	return c.send(ctx, threadID, msgs)
}

func (c *CmdG) send(ctx context.Context, threadID ThreadID, msg string) (err error) {
	return wrapLogRPC("gmail.Users.Messages.Send", func() error {
		_, err = c.gmail.Users.Messages.Send(email, &gmail.Message{
			Raw:      MIMEEncode(msg),
			ThreadId: string(threadID),
		}).Context(ctx).Do()
		return err
	}, "email=%q threadID=%q msg=%q", email, threadID, msg)
}

// PutFile uploads a file into the config dir on Google drive.
func (c *CmdG) PutFile(ctx context.Context, fn string, contents []byte) error {
	const name = "signature.txt"
	const folder = appDataFolder
	err := wrapLogRPC("drive.Files.Create", func() error {
		_, err := c.drive.Files.Create(&drive.File{
			Name:    name,
			Parents: []string{folder},
		}).Context(ctx).Media(bytes.NewBuffer(contents)).Do()
		return err
	}, "name=%q parents=%s", name, folder)
	if err != nil {
		return errors.Wrapf(err, "creating file %q with %d bytes of data", fn, len(contents))
	}
	return nil
}

func (c *CmdG) getFileID(ctx context.Context, fn string) (string, error) {
	var token string
	for {
		var l *drive.FileList
		err := wrapLogRPC("drive.Files.List", func() (err error) {
			l, err = c.drive.Files.List().Context(ctx).Spaces(appDataFolder).PageToken(token).Do()
			return
		}, "folder=%q token=%q", appDataFolder, token)
		if err != nil {
			return "", err
		}
		for _, f := range l.Files {
			if f.Name == fn {
				return f.Id, nil
			}
		}
		token = l.NextPageToken
		if token == "" {
			break
		}
	}
	return "", os.ErrNotExist
}

// UpdateFile updates an existing file on Google drive in the config folder..
func (c *CmdG) UpdateFile(ctx context.Context, fn string, contents []byte) error {
	id, err := c.getFileID(ctx, fn)
	if err != nil {
		if err == os.ErrNotExist {
			if err := wrapLogRPC("drive.Files.Create", func() error {
				_, err := c.drive.Files.Create(&drive.File{
					Name:    fn,
					Parents: []string{appDataFolder},
				}).Context(ctx).Media(bytes.NewBuffer(contents)).Do()
				return err
			}, "name=%q parent=%q", fn, appDataFolder); err != nil {
				return errors.Wrapf(err, "creating file %q with %d bytes of data", fn, len(contents))
			}
			return nil
		}
		return errors.Wrapf(err, "getting file ID for %q", fn)
	}

	if err := wrapLogRPC("drive.Files.Update", func() error {
		_, err := c.drive.Files.Update(id, &drive.File{
			Name: fn,
		}).Context(ctx).Media(bytes.NewBuffer(contents)).Do()
		return err
	}, "name=%q", fn); err != nil {
		return errors.Wrapf(err, "updating file %q, id %q", fn, id)
	}
	return nil
}

// GetFile downloads a file from the config folder.
func (c *CmdG) GetFile(ctx context.Context, fn string) ([]byte, error) {
	var token string
	for {
		var l *drive.FileList
		err := wrapLogRPC("drive.Files.List", func() (err error) {
			l, err = c.drive.Files.List().Context(ctx).Spaces(appDataFolder).PageToken(token).Do()
			return
		}, "spaces=%q token=%q", appDataFolder, token)
		if err != nil {
			return nil, err
		}
		for _, f := range l.Files {
			if f.Name == fn {
				var r *http.Response
				err := wrapLogRPC("drive.Files.Get", func() (err error) {
					r, err = c.drive.Files.Get(f.Id).Context(ctx).Download()
					return
				}, "fileID=%v", f.Id)
				if err != nil {
					return nil, err
				}
				defer r.Body.Close()
				return ioutil.ReadAll(r.Body)
			}
		}
		token = l.NextPageToken
		if token == "" {
			break
		}
	}

	return nil, os.ErrNotExist
}

// Settings stores settings in the app specific folder on Google Drive.
type Settings struct {
	Sender string `json:"sender,omitempty"`
}

// GetDefaultSender gets the default sender, if no From was provided at compose.
func (c *CmdG) GetDefaultSender() string {
	return c.settings.Sender
}

// SetDefaultSender sets default sender, used if the user didn't enter
// a From line manually.
func (c *CmdG) SetDefaultSender(s string) {
	c.settings.Sender = s
}

// LoadSettings loads settings from app specific folder on Google Drive.
func (c *CmdG) LoadSettings(ctx context.Context) error {
	b, err := c.GetFile(ctx, settingsFileName)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &c.settings); err != nil {
		return err
	}
	return nil
}

// SaveSettings saves settings to app specific folder on Google Drive.
func (c *CmdG) SaveSettings(ctx context.Context) error {
	b, err := json.Marshal(c.settings)
	if err != nil {
		return err
	}
	return c.UpdateFile(ctx, settingsFileName, b)
}

// MakeDraft creates a new draft.
func (c *CmdG) MakeDraft(ctx context.Context, msg string) error {
	return wrapLogRPC("gmail.Users.Drafts.Create", func() error {
		_, err := c.gmail.Users.Drafts.Create(email, &gmail.Draft{
			Message: &gmail.Message{
				Raw: MIMEEncode(msg),
			},
		}).Context(ctx).Do()
		return err
	}, "email=%q msg=%q", email, msg)
}

// BatchArchive archives all the given message IDs.
func (c *CmdG) BatchArchive(ctx context.Context, ids []string) error {
	return wrapLogRPC("gmail.Users.Messages.BatchModify", func() error {
		return c.gmail.Users.Messages.BatchModify(email, &gmail.BatchModifyMessagesRequest{
			Ids:            ids,
			RemoveLabelIds: []string{Inbox},
		}).Context(ctx).Do()
	}, "email=%q remove=INBOX ids=%v)", email, ids)
}

// BatchDelete deletes. Does not put in trash. Does not pass go:
// "Immediately and permanently deletes the specified message. This operation cannot be undone."
//
// cmdg doesn't actually request oauth permission to do this, so this function is never used.
// Instead BatchTrash is used.
func (c *CmdG) BatchDelete(ctx context.Context, ids []string) error {
	return wrapLogRPC("gmail.Users.Messages.BatchDelete", func() error {
		return c.gmail.Users.Messages.BatchDelete(email, &gmail.BatchDeleteMessagesRequest{
			Ids: ids,
		}).Context(ctx).Do()
	}, "email=%q ids=%v", email, ids)
}

// BatchTrash trashes the messages.
//
// There isn't actually a BatchTrash, so we'll pretend with just labels.
func (c *CmdG) BatchTrash(ctx context.Context, ids []string) error {
	return c.BatchLabel(ctx, ids, Trash)
}

// BatchLabel adds one new label to many messages.
func (c *CmdG) BatchLabel(ctx context.Context, ids []string, labelID string) error {
	return wrapLogRPC("gmail.Users.Messages.BatchModify", func() error {
		return c.gmail.Users.Messages.BatchModify(email, &gmail.BatchModifyMessagesRequest{
			Ids:         ids,
			AddLabelIds: []string{labelID},
		}).Context(ctx).Do()
	}, "email=%q add_labelID=%v ids=%v", email, labelID, ids)
}

// BatchUnlabel removes one label from many messages.
func (c *CmdG) BatchUnlabel(ctx context.Context, ids []string, labelID string) (err error) {
	return wrapLogRPC("gmail.Users.Messages.BatchModify", func() error {
		return c.gmail.Users.Messages.BatchModify(email, &gmail.BatchModifyMessagesRequest{
			Ids:            ids,
			RemoveLabelIds: []string{labelID},
		}).Context(ctx).Do()
	}, "email=%q remove_labelID=%v ids=%v", email, labelID, ids)
}

// HistoryID returns the current history ID.
func (c *CmdG) HistoryID(ctx context.Context) (HistoryID, error) {
	var p *gmail.Profile
	err := wrapLogRPC("gmail.Users.GetProfile", func() (err error) {
		p, err = c.gmail.Users.GetProfile(email).Context(ctx).Do()
		return
	}, "email=%q", email)
	if err != nil {
		return 0, err
	}
	return HistoryID(p.HistoryId), nil
}

// MoreHistory returns if stuff happened since start ID.
func (c *CmdG) MoreHistory(ctx context.Context, start HistoryID, labelID string) (bool, error) {
	log.Infof("History for %d %s", start, labelID)
	var r *gmail.ListHistoryResponse
	err := wrapLogRPC("gmail.Users.History.List", func() (err error) {

		r, err = c.gmail.Users.History.List(email).Context(ctx).StartHistoryId(uint64(start)).LabelId(labelID).Do()
		return
	}, "email=%q historyID=%v labelID=%q", email, start, labelID)
	if err != nil {
		return false, err
	}
	return len(r.History) > 0, nil
}

// History returns history since startID (all pages).
func (c *CmdG) History(ctx context.Context, startID HistoryID, labelID string) ([]*gmail.History, HistoryID, error) {
	log.Infof("History for %d %s", startID, labelID)
	var ret []*gmail.History
	var h HistoryID
	err := wrapLogRPC("gmail.Users.History.List", func() error {
		return c.gmail.Users.History.List(email).Context(ctx).StartHistoryId(uint64(startID)).LabelId(labelID).Pages(ctx, func(r *gmail.ListHistoryResponse) error {
			ret = append(ret, r.History...)
			h = HistoryID(r.HistoryId)
			return nil
		})
	}, "email=%q historyID=%v labelID=%q", email, startID, labelID)
	if err != nil {
		return nil, 0, err
	}
	return ret, h, nil
}

// ListMessages lists messages in a given label or query, with optional page token.
func (c *CmdG) ListMessages(ctx context.Context, label, query, token string) (*Page, error) {
	const fields = "messages,resultSizeEstimate,nextPageToken"
	nres := int64(pageSize)

	q := c.gmail.Users.Messages.List(email).
		PageToken(token).
		MaxResults(int64(nres)).
		Context(ctx).
		Fields(fields)
	if query != "" {
		q = q.Q(query)
	}
	if label != "" {
		q = q.LabelIds(label)
	}
	var res *gmail.ListMessagesResponse
	err := wrapLogRPC("gmail.Users.Messages.List", func() (err error) {
		res, err = q.Do()
		return
	}, "email=%q token=%v labelID=%q query=%q size=%d fields=%q)", email, token, label, query, nres, fields)
	if err != nil {
		return nil, errors.Wrap(err, "listing messages")
	}
	log.Infof("Next page token: %q", res.NextPageToken)
	p := &Page{
		conn:     c,
		Label:    label,
		Query:    query,
		Response: res,
	}
	for _, m := range res.Messages {
		p.Messages = append(p.Messages, NewMessage(c, m.Id))
	}
	return p, nil
}

// ListDrafts lists all drafts.
func (c *CmdG) ListDrafts(ctx context.Context) ([]*Draft, error) {
	var ret []*Draft
	if err := wrapLogRPC("gmail.Users.Drafts.List", func() error {
		return c.gmail.Users.Drafts.List(email).Pages(ctx, func(r *gmail.ListDraftsResponse) error {
			for _, d := range r.Drafts {
				nd := NewDraft(c, d.Id)
				ret = append(ret, nd)
				go func() {
					if err := nd.load(ctx, LevelMetadata); err != nil {
						log.Errorf("Loading a draft: %v", err)
					}
				}()
			}
			return nil
		})
	}, "email=%q", email); err != nil {
		return nil, err
	}
	return ret, nil
}
