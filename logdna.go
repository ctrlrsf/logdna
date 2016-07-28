package logdna

import "bytes"
import "encoding/json"
import "fmt"
import "net/http"
import "net/url"
import "time"

// IngestBaseURL is the base URL for the LogDNA ingest API.
const IngestBaseURL = "https://logs.logdna.com/logs/ingest"

// DefaultFlushLimit is the number of log lines before we flush to LogDNA
const DefaultFlushLimit = 5000

// Config is used by NewClient to configure new clients.
type Config struct {
	APIKey     string
	LogFile    string
	Hostname   string
	FlushLimit int
}

// A Client is a client to the LogDNA logging service.
type Client struct {
	config  Config
	payload PayloadJSON
	apiURL  url.URL
}

// LogLineJSON represents a log line in the LogDNA ingest API JSON payload.
type LogLineJSON struct {
	Timestamp int64  `json:"timestamp"`
	Line      string `json:"line"`
	File      string `json:"file"`
}

// PayloadJSON is the complete JSON payload that will be sent to the LogDNA
// ingest API.
type PayloadJSON struct {
	Lines []LogLineJSON `json:"lines"`
}

// makeIngestURL creats a new URL to the a full LogDNA ingest API endpoint with
// API key and requierd parameters.
func makeIngestURL(cfg Config) url.URL {
	u, _ := url.Parse(IngestBaseURL)

	u.User = url.User(cfg.APIKey)
	u.RawQuery = fmt.Sprintf("hostname=%s&now=%d", cfg.Hostname, time.Time{}.UnixNano())

	return *u
}

// NewClient returns a Client configured to send logs to the LogDNA ingest API.
func NewClient(cfg Config) *Client {
	if cfg.FlushLimit == 0 {
		cfg.FlushLimit = DefaultFlushLimit
	}

	client := Client{}
	client.apiURL = makeIngestURL(cfg)

	client.config = cfg

	return &client
}

// Log adds a new log line to Client's payload.
//
// To actually send the logs, Flush() needs to be called.
//
// Flush is called automatically if we reach the client's flush limit.
func (c *Client) Log(t time.Time, msg string) {
	if c.Size() == c.config.FlushLimit {
		c.Flush()
	}

	logLine := LogLineJSON{
		Timestamp: t.UnixNano(),
		Line:      msg,
		File:      c.config.LogFile,
	}
	c.payload.Lines = append(c.payload.Lines, logLine)
}

// Size returns the number of lines waiting to be sent.
func (c *Client) Size() int {
	return len(c.payload.Lines)
}

// Flush sends any buffered logs to LogDNA and clears the buffered logs.
func (c *Client) Flush() error {
	// Return immediately if no logs to send
	if c.Size() == 0 {
		return nil
	}

	jsonPayload, err := json.Marshal(c.payload)
	if err != nil {
		return err
	}

	jsonBuffer := bytes.NewBuffer(jsonPayload)

	_, err = http.Post(c.apiURL.String(), "application/json", jsonBuffer)

	if err == nil {
		c.payload = PayloadJSON{}
	}

	return err
}

// Close closes the client. It also sends any buffered logs.
func (c *Client) Close() {
	c.Flush()
}
