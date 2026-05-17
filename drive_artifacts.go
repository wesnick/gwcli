package main

import (
	"context"
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"golang.org/x/net/html"
)

// driveArtifact is a Google Drive document/file linked from an email body
// (e.g. the "Notes by Gemini" / Google Meet artifact chips). It is not a MIME
// attachment; the canonical key is the Drive file ID, from which every URL
// variant is templated.
type driveArtifact struct {
	Index int    `json:"index" yaml:"index"`
	ID    string `json:"id" yaml:"id"`
	Type  string `json:"type" yaml:"type"`
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	URL   string `json:"url" yaml:"url"`
}

// meetArtifactHeader is the header Google sets on Meet/Gemini artifact emails.
// Its base64 payload embeds the canonical Drive file ID, which we use to
// distinguish a genuine artifact from an arbitrary inline docs.google.com link.
const meetArtifactHeader = "X-Meet-Artifact-Email-Metadata"

// driveURLRE matches the Drive/Docs hosted-file URL shapes we care about,
// capturing the product segment and the file ID.
var driveURLRE = regexp.MustCompile(
	`https?://(?:docs|drive)\.google\.com/` +
		`(?:(document|spreadsheets|presentation|forms)/d/([A-Za-z0-9_-]+)` +
		`|file/d/([A-Za-z0-9_-]+)` +
		`|drive/folders/([A-Za-z0-9_-]+)` +
		`|open\?id=([A-Za-z0-9_-]+))`)

// driveTypeByProduct maps the docs.google.com product segment to our type name.
var driveTypeByProduct = map[string]string{
	"document":     "document",
	"spreadsheets": "spreadsheet",
	"presentation": "presentation",
	"forms":        "form",
}

// parseDriveURL extracts the file ID and artifact type from a Drive/Docs URL.
// ok is false if the URL is not a recognized hosted-file link.
func parseDriveURL(rawURL string) (id, typ string, ok bool) {
	m := driveURLRE.FindStringSubmatch(rawURL)
	if m == nil {
		return "", "", false
	}
	switch {
	case m[1] != "" && m[2] != "":
		return m[2], driveTypeByProduct[m[1]], true
	case m[3] != "":
		return m[3], "drive-file", true
	case m[4] != "":
		return m[4], "folder", true
	case m[5] != "":
		return m[5], "drive-file", true
	}
	return "", "", false
}

// canonicalDriveURL builds a clean, stable URL from the ID and type, dropping
// the tracking params and heading fragments present in the scraped links.
func canonicalDriveURL(id, typ string) string {
	switch typ {
	case "document":
		return "https://docs.google.com/document/d/" + id + "/edit"
	case "spreadsheet":
		return "https://docs.google.com/spreadsheets/d/" + id + "/edit"
	case "presentation":
		return "https://docs.google.com/presentation/d/" + id + "/edit"
	case "form":
		return "https://docs.google.com/forms/d/" + id + "/edit"
	case "folder":
		return "https://drive.google.com/drive/folders/" + id
	default:
		return "https://drive.google.com/file/d/" + id + "/view"
	}
}

// decodeMeetArtifactIDs returns the raw decoded bytes (as a string) of the
// Meet artifact metadata header. The header is base64; the file ID appears
// verbatim within the decoded payload, so callers substring-match against it.
func decodeMeetArtifactIDs(headerValue string) string {
	v := strings.TrimSpace(headerValue)
	if v == "" {
		return ""
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(v); err == nil {
			return string(b)
		}
	}
	return ""
}

// anchorText returns the concatenated, trimmed text content of an <a> node.
func anchorText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		for ch := c.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(sb.String()), " ")
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// detectDriveArtifacts parses an HTML body for Drive/Docs links and keeps only
// those that are genuine artifacts: either the file ID appears in the decoded
// Meet metadata header, or the anchor carries Google's artifact chip/button
// class. Results are deduped by file ID with the best available title.
func detectDriveArtifacts(htmlBody, meetHeader string) []driveArtifact {
	if strings.TrimSpace(htmlBody) == "" {
		return nil
	}
	doc, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		return nil
	}
	meetMeta := decodeMeetArtifactIDs(meetHeader)

	// titlePrio ranks where a title came from; higher wins. The artifact chip
	// carries the canonical short label (e.g. "Notes by Gemini"); the
	// link-button is the CTA ("Open meeting notes"); anything else is weakest.
	const (
		prioOther = iota
		prioLinkButton
		prioChip
	)
	type cand struct {
		typ, title string
		titlePrio  int
	}
	byID := map[string]*cand{}
	var order []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			if id, typ, ok := parseDriveURL(href); ok {
				class := attr(n, "class")
				isChip := strings.Contains(class, "artifact-chip")
				isButton := strings.Contains(class, "link-button")
				inMeet := meetMeta != "" && strings.Contains(meetMeta, id)
				if isChip || isButton || inMeet {
					prio := prioOther
					if isButton {
						prio = prioLinkButton
					}
					if isChip {
						prio = prioChip
					}
					title := anchorText(n)
					c := byID[id]
					if c == nil {
						c = &cand{typ: typ}
						byID[id] = c
						order = append(order, id)
					}
					// Keep the highest-priority non-empty title seen.
					if title != "" && (c.title == "" || prio > c.titlePrio) {
						c.title = title
						c.titlePrio = prio
					}
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	var out []driveArtifact
	for i, id := range order {
		c := byID[id]
		out = append(out, driveArtifact{
			Index: i,
			ID:    id,
			Type:  c.typ,
			Title: c.title,
			URL:   canonicalDriveURL(id, c.typ),
		})
	}
	return out
}

// extractDriveArtifacts fetches the message once and returns the Drive
// artifacts linked from its HTML body. It never errors the caller's flow:
// detection is best-effort enrichment.
func extractDriveArtifacts(ctx context.Context, conn *gwcli.CmdG, messageID string) ([]driveArtifact, error) {
	gmailSvc := conn.GmailService()
	msg, err := gmailSvc.Users.Messages.Get("me", messageID).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	var meetHeader string
	if msg.Payload != nil {
		for _, h := range msg.Payload.Headers {
			if strings.EqualFold(h.Name, meetArtifactHeader) {
				meetHeader = h.Value
				break
			}
		}
	}

	htmlBody := extractHTMLFromPart(msg.Payload)
	return detectDriveArtifacts(htmlBody, meetHeader), nil
}
