package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
)

var version = "dev"

type CLI struct {
	Config  string `help:"Config directory path" default:"~/.config/gwcli" type:"path"`
	User    string `help:"User email for service account impersonation (required for service accounts)"`
	JSON    bool   `help:"JSON output format"`
	Verbose bool   `help:"Verbose logging"`
	NoColor bool   `help:"Disable colored output"`

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
	} `cmd:"" help:"Label operations (use gmailctl to create/delete labels)"`

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

	Gmailctl struct {
		Download struct {
			Output string `short:"o" help:"Output file (default: config.jsonnet in config directory)"`
			Yes    bool   `short:"y" help:"Skip overwrite confirmation prompt"`
		} `cmd:"" help:"Download filters from Gmail to config file"`

		Apply struct {
			Yes          bool `short:"y" help:"Skip confirmation prompt"`
			RemoveLabels bool `help:"Allow removing labels not in config" name:"remove-labels"`
		} `cmd:"" help:"Apply config.jsonnet to Gmail filters"`

		Diff struct{} `cmd:"" help:"Show diff between local config and Gmail"`
	} `cmd:"" help:"gmailctl filter management"`

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

	case "gmailctl download":
		cmdCtx := context.Background()
		if err := runGmailctlDownload(cmdCtx, cli.Config, cli.User, cli.Gmailctl.Download.Output, cli.Gmailctl.Download.Yes, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "gmailctl apply":
		cmdCtx := context.Background()
		if err := runGmailctlApply(cmdCtx, cli.Config, cli.User, cli.Gmailctl.Apply.Yes, cli.Gmailctl.Apply.RemoveLabels, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "gmailctl diff":
		cmdCtx := context.Background()
		if err := runGmailctlDiff(cmdCtx, cli.Config, cli.User, out); err != nil {
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
