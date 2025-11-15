package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

var version = "dev"

type CLI struct {
	Config  string `help:"Config directory path" default:"~/.config/gwcli" type:"path"`
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
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("gwcli"),
		kong.Description("Command-line Gmail client"),
		kong.UsageOnError(),
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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
		conn, err := getConnection(cli.Config)
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

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", ctx.Command())
		os.Exit(1)
	}
}
