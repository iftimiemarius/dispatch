package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GraphBase is the Microsoft Graph REST root.
const GraphBase = "https://graph.microsoft.com/v1.0"

// Client makes authenticated Graph calendar calls. Obtain one with
// NewClientFromAuthenticator, which fails if not authenticated.
type Client struct {
	http *http.Client
}

// NewClientFromAuthenticator returns an authenticated client using the given
// authenticator's OAuth config.
func NewClientFromAuthenticator(ctx context.Context, a *Authenticator) (*Client, error) {
	hc, err := HTTPClient(ctx, a.Config)
	if err != nil {
		return nil, err
	}
	return &Client{http: hc}, nil
}

// do performs an authenticated Graph request, decoding JSON into dst (if any).
func (c *Client) do(ctx context.Context, method, path string, body any, dst any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, GraphBase+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("graph %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return parseGraphError(resp)
	}
	if dst != nil {
		return json.NewDecoder(resp.Body).Decode(dst)
	}
	return nil
}

// parseGraphError extracts the error message Microsoft Graph returns.
func parseGraphError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var ge struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &ge); err == nil && ge.Error.Message != "" {
		return fmt.Errorf("graph %s: %s: %s", resp.Status, ge.Error.Code, ge.Error.Message)
	}
	return fmt.Errorf("graph %s: %s", resp.Status, string(data))
}

// --- calendar events ---

// Event is the subset of a Graph calendar event Dispatch syncs.
type Event struct {
	ID        string     `json:"id,omitempty"`
	Subject   string     `json:"subject"`
	Body      eventBody  `json:"body,omitempty"`
	Start     dateTimeTZ `json:"start"`
	End       dateTimeTZ `json:"end"`
	IsAllDay  bool       `json:"isAllDay,omitempty"`
}

type eventBody struct {
	ContentType string `json:"contentType"` // "text" or "HTML"
	Content     string `json:"content"`
}

// dateTimeTZ is Graph's {dateTime, timeZone} pair; times are ISO without zone
// plus an IANA zone name.
type dateTimeTZ struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

// CreateEvent creates a calendar event and returns its Graph id.
func (c *Client) CreateEvent(ctx context.Context, e Event) (string, error) {
	var created Event
	if err := c.do(ctx, http.MethodPost, "/me/events", e, &created); err != nil {
		return "", err
	}
	return created.ID, nil
}

// UpdateEvent patches an existing event by id.
func (c *Client) UpdateEvent(ctx context.Context, id string, e Event) error {
	return c.do(ctx, http.MethodPatch, "/me/events/"+id, e, nil)
}

// DeleteEvent removes an event by id.
func (c *Client) DeleteEvent(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/me/events/"+id, nil, nil)
}

// NewEventFromTimes builds an Event with the given subject, body, and time
// range in the provided (local) zone. If zone is empty it resolves the system
// local zone to an IANA name (Graph rejects the Go "Local" placeholder).
func NewEventFromTimes(subject, body string, start, end time.Time, zone string) Event {
	zone = resolveZone(zone, start.Location())
	return Event{
		Subject: subject,
		Body:    eventBody{ContentType: "text", Content: body},
		Start:   dateTimeTZ{DateTime: start.Format("2006-01-02T15:04:05"), TimeZone: zone},
		End:     dateTimeTZ{DateTime: end.Format("2006-01-02T15:04:05"), TimeZone: zone},
	}
}

// resolveZone returns an IANA timezone name suitable for Graph. It fixes the
// Go "Local"/"" placeholder by resolving the system zone, falling back to UTC
// if the name can't be determined.
func resolveZone(zone string, loc *time.Location) string {
	if zone != "" && zone != "Local" {
		return zone
	}
	if name := loc.String(); name != "" && name != "Local" {
		return name
	}
	if name := systemLocalZone(); name != "" {
		return name
	}
	return "UTC"
}

// systemLocalZone tries to read the system timezone name. On Linux/macOS the
// /etc/localtime symlink points at a zoneinfo file under zoneinfo/<Name>; we
// extract that name. Returns "" if it can't be determined.
func systemLocalZone() string {
	const dir = "/usr/share/zoneinfo/"
	// /etc/localtime is typically a symlink into the zoneinfo tree.
	if link, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(link, "zoneinfo/"); i >= 0 {
			return strings.TrimPrefix(link[i:], "zoneinfo/")
		}
		// Relative symlink (e.g. ../usr/share/zoneinfo/Europe/Bucharest).
		if abs, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
			if i := strings.Index(abs, dir); i >= 0 {
				return strings.TrimPrefix(abs[i:], dir)
			}
		}
	}
	return ""
}

// BlockLike is the block shape the Event mapper needs; decoupled from models
// to avoid an import cycle.
type BlockLike struct {
	Title    string
	Notes    string
	TaskID   *string
	StartsAt time.Time
	EndsAt   time.Time
}

// EventFromBlock maps a block-like value to a Graph event. zone overrides the
// block's location when non-empty.
func EventFromBlock(b BlockLike, zone string) Event {
	subject := b.Title
	body := b.Notes
	if b.TaskID != nil {
		if subject == "" {
			subject = "focus block"
		}
		if body != "" {
			body += "\n\n"
		}
		body += "dispatch task: " + *b.TaskID
	}
	return NewEventFromTimes(subject, body, b.StartsAt, b.EndsAt, zone)
}
