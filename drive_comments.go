package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	drive "google.golang.org/api/drive/v3"
)

// driveCommentFields is the field mask for a single comment. The Drive
// comments/replies endpoints reject requests that do not set fields, so every
// call passes it (or driveCommentListFields / driveReplyFields).
const driveCommentFields = "id,author(displayName,emailAddress),content," +
	"quotedFileContent(value),anchor,resolved,deleted,createdTime,modifiedTime," +
	"replies(" + driveReplyFields + ")"

// driveCommentListFields wraps driveCommentFields for a paginated list call.
const driveCommentListFields = "nextPageToken,comments(" + driveCommentFields + ")"

// driveReplyFields is the field mask for a single reply.
const driveReplyFields = "id,author(displayName,emailAddress),content,action,deleted,createdTime"

// driveCommentAuthor is the trimmed author shape. Drive usually leaves
// emailAddress empty for comment authors, so both fields are optional.
type driveCommentAuthor struct {
	DisplayName  string `json:"displayName,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
}

// driveCommentQuote is the text of the file the comment was anchored to.
type driveCommentQuote struct {
	Value string `json:"value"`
}

// driveCommentReply is the trimmed reply shape emitted by `drive comments`.
type driveCommentReply struct {
	ID          string              `json:"id"`
	Author      *driveCommentAuthor `json:"author,omitempty"`
	Content     string              `json:"content"`
	Action      string              `json:"action,omitempty"`
	Deleted     bool                `json:"deleted,omitempty"`
	CreatedTime string              `json:"createdTime,omitempty"`
}

// driveCommentOutput is the trimmed comment shape emitted by `drive comments`.
// anchor is passed through verbatim (opaque for non-Google files).
type driveCommentOutput struct {
	ID                string              `json:"id"`
	Author            *driveCommentAuthor `json:"author,omitempty"`
	Content           string              `json:"content"`
	QuotedFileContent *driveCommentQuote  `json:"quotedFileContent,omitempty"`
	Anchor            string              `json:"anchor,omitempty"`
	Resolved          bool                `json:"resolved"`
	Deleted           bool                `json:"deleted,omitempty"`
	CreatedTime       string              `json:"createdTime,omitempty"`
	ModifiedTime      string              `json:"modifiedTime,omitempty"`
	Replies           []driveCommentReply `json:"replies"`
}

func shapeDriveCommentAuthor(u *drive.User) *driveCommentAuthor {
	if u == nil || (u.DisplayName == "" && u.EmailAddress == "") {
		return nil
	}
	return &driveCommentAuthor{DisplayName: u.DisplayName, EmailAddress: u.EmailAddress}
}

func shapeDriveReply(r *drive.Reply) driveCommentReply {
	return driveCommentReply{
		ID:          r.Id,
		Author:      shapeDriveCommentAuthor(r.Author),
		Content:     r.Content,
		Action:      r.Action,
		Deleted:     r.Deleted,
		CreatedTime: r.CreatedTime,
	}
}

// shapeDriveComment trims an API comment to the emitted shape. Deleted replies
// are dropped unless includeDeleted is set.
func shapeDriveComment(c *drive.Comment, includeDeleted bool) driveCommentOutput {
	o := driveCommentOutput{
		ID:           c.Id,
		Author:       shapeDriveCommentAuthor(c.Author),
		Content:      c.Content,
		Anchor:       c.Anchor,
		Resolved:     c.Resolved,
		Deleted:      c.Deleted,
		CreatedTime:  c.CreatedTime,
		ModifiedTime: c.ModifiedTime,
		Replies:      []driveCommentReply{},
	}
	if c.QuotedFileContent != nil && c.QuotedFileContent.Value != "" {
		o.QuotedFileContent = &driveCommentQuote{Value: c.QuotedFileContent.Value}
	}
	for _, r := range c.Replies {
		if r.Deleted && !includeDeleted {
			continue
		}
		o.Replies = append(o.Replies, shapeDriveReply(r))
	}
	return o
}

// listDriveComments pages through every comment on a file. includeDeleted is
// passed to the API; resolved comments are always returned (callers filter).
func listDriveComments(ctx context.Context, svc *drive.Service, fileID string, includeDeleted bool) ([]*drive.Comment, error) {
	var comments []*drive.Comment
	pageToken := ""
	for {
		call := svc.Comments.List(fileID).
			Fields(driveCommentListFields).
			IncludeDeleted(includeDeleted).
			PageSize(100).
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, wrapDriveErr(err)
		}
		comments = append(comments, resp.Comments...)
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return comments, nil
}

// filterDriveComments shapes comments for output, dropping resolved ones
// unless includeResolved and deleted ones unless includeDeleted.
func filterDriveComments(comments []*drive.Comment, includeResolved, includeDeleted bool) []driveCommentOutput {
	shaped := []driveCommentOutput{}
	for _, c := range comments {
		if c.Resolved && !includeResolved {
			continue
		}
		if c.Deleted && !includeDeleted {
			continue
		}
		shaped = append(shaped, shapeDriveComment(c, includeDeleted))
	}
	return shaped
}

// countOpenDriveComments returns how many comments are neither resolved nor
// deleted.
func countOpenDriveComments(comments []*drive.Comment) int {
	return len(filterDriveComments(comments, false, false))
}

// readCommentContent returns content, or all of r (trailing newlines trimmed)
// when content is "-".
func readCommentContent(content string, r io.Reader) (string, error) {
	if content != "-" {
		return content, nil
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read comment content from stdin: %w", err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

// oneLine flattens whitespace so multi-line comment text fits a table cell.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// runDriveCommentsList lists a file's comments (open ones by default).
func runDriveCommentsList(ctx context.Context, conn *gwcli.CmdG, ref string, includeResolved, includeDeleted bool, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	comments, err := listDriveComments(ctx, svc, art.ID, includeDeleted)
	if err != nil {
		return err
	}
	shaped := filterDriveComments(comments, includeResolved, includeDeleted)
	if len(shaped) == 0 {
		return out.WriteEmptyList("No comments found")
	}
	if out.json {
		return out.writeJSON(shaped)
	}
	headers := []string{"ID", "AUTHOR", "RESOLVED", "REPLIES", "CREATED", "CONTENT"}
	rows := make([][]string, len(shaped))
	for i, c := range shaped {
		author := ""
		if c.Author != nil {
			author = c.Author.DisplayName
			if author == "" {
				author = c.Author.EmailAddress
			}
		}
		resolved := "no"
		if c.Resolved {
			resolved = "yes"
		}
		rows[i] = []string{
			c.ID,
			author,
			resolved,
			fmt.Sprintf("%d", len(c.Replies)),
			c.CreatedTime,
			truncateString(oneLine(c.Content), 60),
		}
	}
	return out.writeTable(headers, rows)
}

// runDriveCommentsAdd posts an unanchored comment on a file.
func runDriveCommentsAdd(ctx context.Context, conn *gwcli.CmdG, ref, content string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("comment content (--content) is required")
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	created, err := svc.Comments.Create(art.ID, &drive.Comment{Content: content}).
		Fields(driveCommentFields).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	if out.json {
		return out.writeJSON(shapeDriveComment(created, false))
	}
	out.writeMessage(fmt.Sprintf("Added comment %s on %s", created.Id, art.ID))
	return nil
}

// runDriveCommentsReply replies to a comment. With resolve the reply carries
// action=resolve, which marks the comment resolved; content may then be empty.
func runDriveCommentsReply(ctx context.Context, conn *gwcli.CmdG, ref, commentID, content string, resolve bool, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(commentID) == "" {
		return fmt.Errorf("a comment ID is required")
	}
	if strings.TrimSpace(content) == "" && !resolve {
		return fmt.Errorf("reply content (--content) is required unless resolving")
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	reply := &drive.Reply{Content: content}
	if resolve {
		reply.Action = "resolve"
	}
	created, err := svc.Replies.Create(art.ID, commentID, reply).
		Fields(driveReplyFields).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	if out.json {
		return out.writeJSON(map[string]interface{}{
			"fileId":    art.ID,
			"commentId": commentID,
			"reply":     shapeDriveReply(created),
			"resolved":  created.Action == "resolve",
		})
	}
	if resolve {
		out.writeMessage(fmt.Sprintf("Resolved comment %s on %s (reply %s)", commentID, art.ID, created.Id))
		return nil
	}
	out.writeMessage(fmt.Sprintf("Replied to comment %s on %s (reply %s)", commentID, art.ID, created.Id))
	return nil
}
