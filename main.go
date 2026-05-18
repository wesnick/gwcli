package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/alecthomas/kong"
)

var version = "dev"

func init() {
	// If version was set via ldflags (release builds), keep it as-is.
	if version != "dev" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	// Tagged install: "go install github.com/...@v1.2.3" embeds the tag here.
	// Only accept a clean semver tag (e.g., "v1.2.3"). Ignore "(devel)" and
	// pseudo-versions like "v0.0.0-20210101..." which indicate local builds
	// or builds from non-tagged commits.
	if v := info.Main.Version; v != "" && v != "(devel)" && !strings.Contains(v, "-0.") {
		version = v
		return
	}

	// Untagged/dev build: fall back to the VCS commit hash embedded by
	// "go build" when run inside a git checkout.
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		version = rev
		if modified == "true" {
			version += "-dirty"
		}
	}
}

type CLI struct {
	Config  string `help:"Config directory path" default:"~/.config/gwcli" type:"path"`
	User    string `help:"User email for service account impersonation (required for service accounts)"`
	JSON    bool   `help:"JSON output format"`
	Verbose bool   `help:"Verbose logging"`
	NoColor bool   `help:"Disable colored output"`

	VersionFlag kong.VersionFlag `name:"version" short:"V" help:"Print version and exit"`

	Configure struct{} `cmd:"" help:"Configure OAuth authentication"`
	Version   struct{} `cmd:"" help:"Show version"`

	Auth struct {
		TokenInfo struct{} `cmd:"" aliases:"token-info" help:"Show OAuth token information and scopes"`
	} `cmd:"" help:"Authentication operations"`

	Messages struct {
		List struct {
			Label      string `help:"Label to list" default:"INBOX"`
			Limit      int    `help:"Max messages" default:"50"`
			UnreadOnly bool   `help:"Unread only" name:"unread-only"`
		} `cmd:"" help:"List messages"`

		Read struct {
			MessageID   string `arg:"" required:"" help:"Message ID"`
			Raw         bool   `help:"Output RFC822 format"`
			HeadersOnly bool   `help:"Show headers only" name:"headers-only"`
			RawHTML     bool   `help:"Output raw HTML with HTML-formatted metadata" name:"raw-html"`
			PreferPlain bool   `help:"Prefer plain text body over HTML" name:"prefer-plain"`
		} `cmd:"" help:"Read message"`

		Search struct {
			Query string `arg:"" required:"" help:"Gmail search query"`
			Limit int    `help:"Max results" default:"100"`
		} `cmd:"" help:"Search messages"`

		Send struct {
			To       []string `required:"" help:"Recipients"`
			Subject  string   `required:"" help:"Subject line"`
			Body     string   `help:"Message body (or read from stdin)"`
			Cc       []string `help:"CC recipients"`
			Bcc      []string `help:"BCC recipients"`
			Attach   []string `help:"File attachments" type:"existingfile"`
			HTML     bool     `help:"Send as HTML"`
			ThreadID string   `help:"Reply to thread" name:"thread-id"`
		} `cmd:"" help:"Send email"`

		Delete struct {
			MessageID string `arg:"" optional:"" help:"Message ID"`
			Stdin     bool   `help:"Read IDs from stdin"`
			Force     bool   `required:"" help:"Confirm deletion"`
		} `cmd:"" help:"Delete messages"`

		MarkRead struct {
			MessageID string `arg:"" optional:"" help:"Message ID"`
			Stdin     bool   `help:"Read IDs from stdin"`
		} `cmd:"" aliases:"mark-read" help:"Mark as read"`

		MarkUnread struct {
			MessageID string `arg:"" optional:"" help:"Message ID"`
			Stdin     bool   `help:"Read IDs from stdin"`
		} `cmd:"" aliases:"mark-unread" help:"Mark as unread"`

		Move struct {
			MessageID string `arg:"" optional:"" help:"Message ID"`
			To        string `required:"" help:"Destination label"`
			Stdin     bool   `help:"Read IDs from stdin"`
		} `cmd:"" help:"Move to label"`
	} `cmd:"" help:"Message operations"`

	Labels struct {
		List struct {
			System   bool `help:"System labels only"`
			UserOnly bool `help:"User labels only" name:"user-only"`
		} `cmd:"" help:"List labels"`

		Apply struct {
			LabelID   string `arg:"" required:"" help:"Label ID or name"`
			MessageID string `help:"Message ID" name:"message"`
			Stdin     bool   `help:"Read IDs from stdin"`
		} `cmd:"" help:"Apply label to messages"`

		Remove struct {
			LabelID   string `arg:"" required:"" help:"Label ID or name"`
			MessageID string `help:"Message ID" name:"message"`
			Stdin     bool   `help:"Read IDs from stdin"`
		} `cmd:"" help:"Remove label from messages"`
	} `cmd:"" help:"Label operations"`

	Attachments struct {
		List struct {
			MessageID string `arg:"" required:"" help:"Message ID"`
		} `cmd:"" help:"List attachments"`

		Download struct {
			MessageID string   `arg:"" required:"" help:"Message ID"`
			Index     []string `help:"Attachment index (0-based, supports comma-separated and multiple flags)" short:"i"`
			Filename  string   `help:"Filename pattern (glob)" short:"f"`
			OutputDir string   `help:"Output directory" type:"path" default:"~/Downloads"`
			Output    string   `help:"Output filename (single attachment only)"`
		} `cmd:"" help:"Download attachments"`
	} `cmd:"" help:"Attachment operations"`

	Artifacts struct {
		List struct {
			MessageID string `arg:"" required:"" help:"Message ID"`
		} `cmd:"" help:"List Google Drive artifacts linked from a message"`

		Download struct {
			MessageID string   `arg:"" required:"" help:"Message ID"`
			Index     []string `help:"Artifact index (0-based, supports comma-separated and multiple flags)" short:"i"`
			Filename  string   `help:"Title pattern (glob)" short:"f"`
			OutputDir string   `help:"Output directory" type:"path" default:"~/Downloads"`
			Output    string   `help:"Output filename (single artifact only)"`
		} `cmd:"" help:"Download/export Google Drive artifacts linked from a message"`
	} `cmd:"" help:"Google Drive artifact operations (Gemini/Meet doc links)"`

	Drive struct {
		Get struct {
			Ref string `arg:"" required:"" name:"file" help:"Drive file ID or Drive/Docs URL"`
		} `cmd:"" help:"Show Google Drive file metadata"`

		Export struct {
			Ref          string `arg:"" required:"" name:"file" help:"Drive file ID or Drive/Docs URL"`
			ExportFormat string `name:"export-format" help:"Override export format for native docs (alias e.g. pdf,md,docx,csv,xlsx or a raw MIME type)"`
			OutputDir    string `help:"Output directory" type:"path" default:"~/Downloads"`
			Output       string `help:"Output filename"`
		} `cmd:"" help:"Export/download a Google Drive file by ID or URL"`

		List struct {
			Query  string `name:"query" short:"q" help:"Raw Drive query expression (q parameter)"`
			Folder string `name:"folder" help:"Restrict to direct children of this folder ID"`
			Limit  int    `name:"limit" default:"100" help:"Max results (0 = no limit)"`
		} `cmd:"" help:"List Drive files"`

		Search struct {
			Term  string `arg:"" required:"" help:"Search term (matched against name and full text)"`
			Limit int    `name:"limit" default:"100" help:"Max results (0 = no limit)"`
		} `cmd:"" help:"Search Drive files by name/content"`

		Upload struct {
			Paths   []string `arg:"" required:"" name:"path" help:"Local file(s) or directory to upload"`
			Folder  string   `name:"folder" help:"Destination folder ID or URL"`
			Name    string   `name:"name" help:"Override the uploaded file name (single file only)"`
			Convert bool     `name:"convert" help:"Convert to the native Google-apps type by extension (csv->Sheet, md->Doc, ...)"`
			As      string   `name:"as" help:"Force the converted-to Google type, overriding extension inference: doc|sheet|slides|drawing|form or a raw application/vnd.google-apps.* type"`
			Upsert  bool     `name:"upsert" help:"Replace an existing same-name file in the destination instead of creating a duplicate"`
		} `cmd:"" help:"Upload local file(s)/directory to Drive"`

		Update struct {
			Ref  string `arg:"" required:"" name:"file" help:"Drive file ID or Drive/Docs URL"`
			Path string `arg:"" required:"" name:"path" type:"existingfile" help:"Local file with new content"`
			Name string `name:"name" help:"Also rename the file"`
		} `cmd:"" help:"Replace an existing Drive file's content"`

		Mkdir struct {
			Name     string `arg:"" required:"" name:"name" help:"Folder name to create"`
			Folder   string `name:"folder" help:"Parent folder ID or URL (default: My Drive root)"`
			NoDedupe bool   `name:"no-dedupe" help:"Always create a new folder even if one with this name already exists in the parent"`
		} `cmd:"" help:"Create a Drive folder (reuses an existing same-name folder by default)"`

		Mv struct {
			Ref    string `arg:"" required:"" name:"file" help:"Drive file ID or URL to move"`
			Folder string `name:"folder" required:"" help:"Destination folder ID or URL"`
		} `cmd:"" help:"Move (reparent) a Drive file into a folder"`

		Rename struct {
			Ref  string `arg:"" required:"" name:"file" help:"Drive file ID or URL"`
			Name string `arg:"" required:"" name:"name" help:"New name"`
		} `cmd:"" help:"Rename a Drive file (content untouched)"`

		Cp struct {
			Ref    string `arg:"" required:"" name:"file" help:"Drive file ID or URL to copy"`
			Name   string `name:"name" help:"Name for the copy"`
			Folder string `name:"folder" help:"Destination folder ID or URL"`
		} `cmd:"" help:"Copy a Drive file (template instantiation)"`

		Rm struct {
			Ref       string `arg:"" required:"" name:"file" help:"Drive file ID or URL"`
			Permanent bool   `name:"permanent" help:"Skip the trash and delete irreversibly"`
			Force     bool   `name:"force" short:"f" help:"Confirm deletion"`
		} `cmd:"" help:"Trash (default) or permanently delete a Drive file"`

		Share struct {
			Ref     string `arg:"" required:"" name:"file" help:"Drive file ID or URL"`
			Type    string `name:"type" default:"user" help:"Principal type: user, group, domain, anyone"`
			Role    string `name:"role" default:"reader" help:"Role: reader, commenter, writer, owner"`
			Email   string `name:"email" help:"Email address (for type user/group)"`
			Domain  string `name:"domain" help:"Domain name (for type domain)"`
			Notify  bool   `name:"notify" help:"Send a notification email"`
			Message string `name:"message" help:"Message included in the notification email"`
		} `cmd:"" help:"Grant a permission on a Drive file"`

		Link struct {
			Ref      string `arg:"" required:"" name:"file" help:"Drive file ID or URL"`
			Role     string `name:"role" default:"reader" help:"Link-sharing role: reader, commenter, writer"`
			NoAnyone bool   `name:"no-anyone" help:"Don't change sharing; just print the existing link"`
		} `cmd:"" help:"Get a shareable link (enables anyone-with-link by default)"`

		Permissions struct {
			Ref string `arg:"" required:"" name:"file" help:"Drive file ID or URL"`
		} `cmd:"" help:"List who can access a Drive file"`
	} `cmd:"" help:"Google Drive operations"`

	Filters struct {
		List struct{} `cmd:"" help:"List all Gmail filters"`

		Get struct {
			FilterID string `arg:"" name:"filter-id" help:"ID of the filter to show"`
		} `cmd:"" help:"Show a single filter's details"`

		Create filtersCreateCmd `cmd:"" help:"Create a new filter"`

		Delete struct {
			FilterID string `arg:"" name:"filter-id" help:"ID of the filter to delete"`
			Force    bool   `name:"force" short:"f" help:"Skip confirmation"`
		} `cmd:"" help:"Delete a filter"`
	} `cmd:"" help:"Manage Gmail filters"`

	Tasklists struct {
		List struct{} `cmd:"" help:"List all task lists"`

		Create struct {
			Title string `arg:"" help:"Title for the new task list"`
		} `cmd:"" help:"Create a new task list"`

		Delete struct {
			TasklistID string `arg:"" name:"tasklist-id" help:"ID of the task list to delete"`
			Force      bool   `name:"force" short:"f" help:"Skip confirmation"`
		} `cmd:"" help:"Delete a task list"`
	} `cmd:"" help:"Manage Google Tasks lists"`

	Tasks struct {
		List struct {
			TasklistID       string `arg:"" name:"tasklist-id" help:"Task list ID"`
			IncludeCompleted bool   `name:"include-completed" short:"a" help:"Include completed tasks"`
		} `cmd:"" help:"List tasks in a task list"`

		Create struct {
			TasklistID string `arg:"" name:"tasklist-id" help:"Task list ID"`
			Title      string `name:"title" short:"t" required:"" help:"Task title"`
			Notes      string `name:"notes" short:"n" help:"Task notes"`
			Due        string `name:"due" short:"d" help:"Due date (RFC3339 format)"`
		} `cmd:"" help:"Create a new task"`

		Read struct {
			TasklistID string `arg:"" name:"tasklist-id" help:"Task list ID"`
			TaskID     string `arg:"" name:"task-id" help:"Task ID"`
		} `cmd:"" help:"Get task details"`

		Complete struct {
			TasklistID string `arg:"" name:"tasklist-id" help:"Task list ID"`
			TaskID     string `arg:"" name:"task-id" help:"Task ID"`
		} `cmd:"" help:"Mark a task as completed"`

		Delete struct {
			TasklistID string `arg:"" name:"tasklist-id" help:"Task list ID"`
			TaskID     string `arg:"" name:"task-id" help:"Task ID"`
			Force      bool   `name:"force" short:"f" help:"Skip confirmation"`
		} `cmd:"" help:"Delete a task"`
	} `cmd:"" help:"Manage Google Tasks"`

	Calendars struct {
		List struct {
			MinAccessRole string `name:"min-access-role" help:"Minimum access role (freeBusyReader, reader, writer, owner)"`
		} `cmd:"" help:"List all accessible calendars"`
	} `cmd:"" help:"Google Calendar operations"`

	Events struct {
		List struct {
			CalendarID   string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
			TimeMin      string `name:"time-min" help:"Start time (RFC3339)"`
			TimeMax      string `name:"time-max" help:"End time (RFC3339)"`
			Query        string `name:"query" short:"q" help:"Search query"`
			MaxResults   int    `name:"max-results" default:"25" help:"Maximum events to return"`
			SingleEvents bool   `name:"single-events" default:"true" help:"Expand recurring events"`
		} `cmd:"" help:"List events in a calendar"`

		Read struct {
			EventID    string `arg:"" required:"" help:"Event ID"`
			CalendarID string `name:"calendar" short:"c" help:"Calendar ID (default: primary)"`
		} `cmd:"" help:"Get event details"`

		Create struct {
			CalendarID  string   `arg:"" optional:"" help:"Calendar ID (default: primary)"`
			Summary     string   `name:"summary" short:"s" required:"" help:"Event title"`
			Description string   `name:"description" short:"d" help:"Event description"`
			Location    string   `name:"location" short:"l" help:"Event location"`
			Start       string   `name:"start" required:"" help:"Start time (RFC3339 or YYYY-MM-DD for all-day)"`
			End         string   `name:"end" help:"End time (RFC3339 or YYYY-MM-DD, default: start + 1 hour)"`
			AllDay      bool     `name:"all-day" help:"Create all-day event"`
			Attendees   []string `name:"attendee" short:"a" help:"Attendee email (can repeat)"`
			Reminders   []string `name:"reminder" short:"r" help:"Reminder spec (e.g., '15m popup', '1h email')"`
			ColorID     string   `name:"color" help:"Event color ID"`
		} `cmd:"" help:"Create a new event"`

		QuickAdd struct {
			Text       string `arg:"" required:"" help:"Natural language event description"`
			CalendarID string `name:"calendar" short:"c" help:"Calendar ID (default: primary)"`
		} `cmd:"" name:"quickadd" help:"Create event from natural language"`

		Update struct {
			EventID     string `arg:"" required:"" help:"Event ID to update"`
			CalendarID  string `name:"calendar" short:"c" help:"Calendar ID (default: primary)"`
			Summary     string `name:"summary" short:"s" help:"New event title"`
			Description string `name:"description" short:"d" help:"New event description"`
			Location    string `name:"location" short:"l" help:"New event location"`
			Start       string `name:"start" help:"New start time (RFC3339)"`
			End         string `name:"end" help:"New end time (RFC3339)"`
			ColorID     string `name:"color" help:"New event color ID"`
		} `cmd:"" help:"Update an event"`

		Delete struct {
			EventID    string `arg:"" required:"" help:"Event ID to delete"`
			CalendarID string `name:"calendar" short:"c" help:"Calendar ID (default: primary)"`
			Force      bool   `name:"force" short:"f" help:"Skip confirmation"`
		} `cmd:"" help:"Delete an event"`

		Search struct {
			Query       string   `arg:"" required:"" help:"Search query"`
			CalendarIDs []string `name:"calendar" short:"c" help:"Calendar IDs to search (can repeat)"`
			TimeMin     string   `name:"time-min" help:"Start time (RFC3339)"`
			TimeMax     string   `name:"time-max" help:"End time (RFC3339)"`
			MaxResults  int      `name:"max-results" default:"25" help:"Maximum events to return"`
		} `cmd:"" help:"Search for events across calendars"`

		Updated struct {
			CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
			UpdatedMin string `name:"updated-min" required:"" help:"Return events updated after this time (RFC3339)"`
			TimeMin    string `name:"time-min" help:"Start time (RFC3339)"`
			TimeMax    string `name:"time-max" help:"End time (RFC3339)"`
			MaxResults int    `name:"max-results" default:"25" help:"Maximum events to return"`
		} `cmd:"" help:"Find events modified since a timestamp"`

		Conflicts struct {
			CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
			TimeMin    string `name:"time-min" help:"Start time (RFC3339, default: now)"`
			TimeMax    string `name:"time-max" help:"End time (RFC3339, default: now + 30 days)"`
		} `cmd:"" help:"Detect scheduling conflicts"`

		Import struct {
			CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
			File       string `name:"file" short:"f" required:"" help:"ICS file path (- for stdin)"`
			DryRun     bool   `name:"dry-run" help:"Parse and validate without importing"`
		} `cmd:"" help:"Import events from ICS file"`
	} `cmd:"" help:"Google Calendar event operations"`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("gwcli"),
		kong.Description("Command-line Gmail client"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{"version": version},
	)

	out := newOutputWriter(cli.JSON, cli.NoColor, cli.Verbose)

	switch ctx.Command() {
	case "configure":
		if err := runConfigure(cli.Config); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(3)
		}

	case "version":
		fmt.Printf("gwcli %s\n", version)

	case "auth token-info":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runAuthTokenInfo(cmdCtx, conn, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages list":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesList(cmdCtx, conn, cli.Messages.List.Label, cli.Messages.List.Limit, cli.Messages.List.UnreadOnly, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages read <message-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesRead(cmdCtx, conn, cli.Messages.Read.MessageID, cli.Messages.Read.Raw, cli.Messages.Read.HeadersOnly, cli.Messages.Read.RawHTML, cli.Messages.Read.PreferPlain, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages search <query>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesSearch(cmdCtx, conn, cli.Messages.Search.Query, cli.Messages.Search.Limit, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages send":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesSend(cmdCtx, conn, cli.Messages.Send.To, cli.Messages.Send.Cc, cli.Messages.Send.Bcc,
			cli.Messages.Send.Subject, cli.Messages.Send.Body, cli.Messages.Send.Attach,
			cli.Messages.Send.HTML, cli.Messages.Send.ThreadID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages delete":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesDelete(cmdCtx, conn, cli.Messages.Delete.MessageID, cli.Messages.Delete.Stdin, cli.Verbose, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages mark-read":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesMarkRead(cmdCtx, conn, cli.Messages.MarkRead.MessageID, cli.Messages.MarkRead.Stdin, cli.Verbose, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages mark-unread":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesMarkUnread(cmdCtx, conn, cli.Messages.MarkUnread.MessageID, cli.Messages.MarkUnread.Stdin, cli.Verbose, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "messages move":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runMessagesMove(cmdCtx, conn, cli.Messages.Move.MessageID, cli.Messages.Move.To, cli.Messages.Move.Stdin, cli.Verbose, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "labels list":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runLabelsList(cmdCtx, conn, cli.Labels.List.System, cli.Labels.List.UserOnly, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "labels apply":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runLabelsApply(cmdCtx, conn, cli.Labels.Apply.LabelID, cli.Labels.Apply.MessageID, cli.Labels.Apply.Stdin, cli.Verbose, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "labels remove":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runLabelsRemove(cmdCtx, conn, cli.Labels.Remove.LabelID, cli.Labels.Remove.MessageID, cli.Labels.Remove.Stdin, cli.Verbose, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "attachments list <message-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runAttachmentsList(cmdCtx, conn, cli.Attachments.List.MessageID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "attachments download <message-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runAttachmentsDownload(cmdCtx, conn, cli.Attachments.Download.MessageID,
			cli.Attachments.Download.Index, cli.Attachments.Download.Filename,
			cli.Attachments.Download.OutputDir, cli.Attachments.Download.Output, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "artifacts list <message-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runArtifactsList(cmdCtx, conn, cli.Artifacts.List.MessageID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "artifacts download <message-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}

		if err := runArtifactsDownload(cmdCtx, conn, cli.Artifacts.Download.MessageID,
			cli.Artifacts.Download.Index, cli.Artifacts.Download.Filename,
			cli.Artifacts.Download.OutputDir, cli.Artifacts.Download.Output, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive get <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveGet(cmdCtx, conn, cli.Drive.Get.Ref, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive export <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveExport(cmdCtx, conn, cli.Drive.Export.Ref,
			cli.Drive.Export.ExportFormat, cli.Drive.Export.OutputDir,
			cli.Drive.Export.Output, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive list":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveList(cmdCtx, conn, cli.Drive.List.Query,
			cli.Drive.List.Folder, cli.Drive.List.Limit, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive search <term>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveSearch(cmdCtx, conn, cli.Drive.Search.Term,
			cli.Drive.Search.Limit, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive upload <path>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveUpload(cmdCtx, conn, cli.Drive.Upload.Paths,
			cli.Drive.Upload.Folder, cli.Drive.Upload.Name,
			cli.Drive.Upload.Convert, cli.Drive.Upload.Upsert,
			cli.Drive.Upload.As, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive update <file> <path>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveUpdate(cmdCtx, conn, cli.Drive.Update.Ref,
			cli.Drive.Update.Path, cli.Drive.Update.Name, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive mkdir <name>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveMkdir(cmdCtx, conn, cli.Drive.Mkdir.Name,
			cli.Drive.Mkdir.Folder, cli.Drive.Mkdir.NoDedupe, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive mv <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveMove(cmdCtx, conn, cli.Drive.Mv.Ref,
			cli.Drive.Mv.Folder, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive rename <file> <name>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveRename(cmdCtx, conn, cli.Drive.Rename.Ref,
			cli.Drive.Rename.Name, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive cp <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveCopy(cmdCtx, conn, cli.Drive.Cp.Ref,
			cli.Drive.Cp.Name, cli.Drive.Cp.Folder, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive rm <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveRm(cmdCtx, conn, cli.Drive.Rm.Ref,
			cli.Drive.Rm.Permanent, cli.Drive.Rm.Force, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive share <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		principal := cli.Drive.Share.Email
		if cli.Drive.Share.Type == "domain" {
			principal = cli.Drive.Share.Domain
		}
		if err := runDriveShare(cmdCtx, conn, cli.Drive.Share.Ref,
			cli.Drive.Share.Type, cli.Drive.Share.Role, principal,
			cli.Drive.Share.Message, cli.Drive.Share.Notify, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive link <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDriveLink(cmdCtx, conn, cli.Drive.Link.Ref,
			cli.Drive.Link.Role, cli.Drive.Link.NoAnyone, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "drive permissions <file>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runDrivePermissions(cmdCtx, conn, cli.Drive.Permissions.Ref, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "filters list":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runFiltersList(cmdCtx, conn, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "filters get <filter-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runFiltersGet(cmdCtx, conn, cli.Filters.Get.FilterID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "filters create":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runFiltersCreate(cmdCtx, conn, cli.Filters.Create, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "filters delete <filter-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runFiltersDelete(cmdCtx, conn, cli.Filters.Delete.FilterID, cli.Filters.Delete.Force, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasklists list":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasklistsList(cmdCtx, conn, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasklists create <title>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasklistsCreate(cmdCtx, conn, cli.Tasklists.Create.Title, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasklists delete <tasklist-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasklistsDelete(cmdCtx, conn, cli.Tasklists.Delete.TasklistID, cli.Tasklists.Delete.Force, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks list <tasklist-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksList(cmdCtx, conn, cli.Tasks.List.TasklistID, cli.Tasks.List.IncludeCompleted, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks create <tasklist-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksCreate(cmdCtx, conn, cli.Tasks.Create.TasklistID, cli.Tasks.Create.Title, cli.Tasks.Create.Notes, cli.Tasks.Create.Due, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks read <tasklist-id> <task-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksRead(cmdCtx, conn, cli.Tasks.Read.TasklistID, cli.Tasks.Read.TaskID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks complete <tasklist-id> <task-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksComplete(cmdCtx, conn, cli.Tasks.Complete.TasklistID, cli.Tasks.Complete.TaskID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks delete <tasklist-id> <task-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksDelete(cmdCtx, conn, cli.Tasks.Delete.TasklistID, cli.Tasks.Delete.TaskID, cli.Tasks.Delete.Force, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	// Calendar commands
	case "calendars list":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runCalendarsList(cmdCtx, conn, cli.Calendars.List.MinAccessRole, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	// Event commands
	case "events list", "events list <calendar-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runEventsList(cmdCtx, conn, cli.Events.List.CalendarID,
			cli.Events.List.TimeMin, cli.Events.List.TimeMax, cli.Events.List.Query,
			cli.Events.List.MaxResults, cli.Events.List.SingleEvents, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events read <event-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runEventsRead(cmdCtx, conn, cli.Events.Read.CalendarID,
			cli.Events.Read.EventID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events create", "events create <calendar-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		opts := createEventOptions{
			summary:     cli.Events.Create.Summary,
			description: cli.Events.Create.Description,
			location:    cli.Events.Create.Location,
			start:       cli.Events.Create.Start,
			end:         cli.Events.Create.End,
			allDay:      cli.Events.Create.AllDay,
			attendees:   cli.Events.Create.Attendees,
			reminders:   cli.Events.Create.Reminders,
			colorID:     cli.Events.Create.ColorID,
		}
		if err := runEventsCreate(cmdCtx, conn, cli.Events.Create.CalendarID, opts, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events quickadd <text>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runEventsQuickAdd(cmdCtx, conn, cli.Events.QuickAdd.CalendarID,
			cli.Events.QuickAdd.Text, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events update <event-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		opts := updateEventOptions{
			summary:     cli.Events.Update.Summary,
			description: cli.Events.Update.Description,
			location:    cli.Events.Update.Location,
			start:       cli.Events.Update.Start,
			end:         cli.Events.Update.End,
			colorID:     cli.Events.Update.ColorID,
		}
		if err := runEventsUpdate(cmdCtx, conn, cli.Events.Update.CalendarID,
			cli.Events.Update.EventID, opts, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events delete <event-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runEventsDelete(cmdCtx, conn, cli.Events.Delete.CalendarID,
			cli.Events.Delete.EventID, cli.Events.Delete.Force, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events search <query>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runEventsSearch(cmdCtx, conn, cli.Events.Search.CalendarIDs,
			cli.Events.Search.Query, cli.Events.Search.TimeMin, cli.Events.Search.TimeMax,
			cli.Events.Search.MaxResults, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events updated", "events updated <calendar-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runEventsUpdated(cmdCtx, conn, cli.Events.Updated.CalendarID,
			cli.Events.Updated.UpdatedMin, cli.Events.Updated.TimeMin, cli.Events.Updated.TimeMax,
			cli.Events.Updated.MaxResults, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events conflicts", "events conflicts <calendar-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runEventsConflicts(cmdCtx, conn, cli.Events.Conflicts.CalendarID,
			cli.Events.Conflicts.TimeMin, cli.Events.Conflicts.TimeMax, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "events import", "events import <calendar-id>":
		cmdCtx := context.Background()
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		var reader io.Reader
		if cli.Events.Import.File == "-" {
			reader = os.Stdin
		} else {
			f, err := os.Open(cli.Events.Import.File)
			if err != nil {
				out.writeError(fmt.Errorf("failed to open file: %w", err))
				os.Exit(2)
			}
			defer f.Close()
			reader = f
		}
		if err := runEventsImport(cmdCtx, conn, cli.Events.Import.CalendarID,
			reader, cli.Events.Import.DryRun, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", ctx.Command())
		os.Exit(1)
	}
}
