package logdna

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

// IngestBaseURL is the base URL for the LogDNA ingest API.
const IngestBaseURL = "https://logs.logdna.com/logs/ingest"

// DefaultFlushLimit is the number of log lines before we flush to LogDNA.
const DefaultFlushLimit = 500

// Config is used by NewClient to configure new clients.
type Config struct {
	APIKey     string
	Hostname   string
	FlushLimit int
}

// logLineJSON represents a log line in the LogDNA ingest API JSON payload.
type logLineJSON struct {
	Timestamp int64  `json:"timestamp"`
	Line      string `json:"line"`
	File      string `json:"file"`
}

// payloadJSON is the complete JSON payload that will be sent to the LogDNA
// ingest API.
type payloadJSON struct {
	Lines []logLineJSON `json:"lines"`
}

// Client is a client to the LogDNA logging service.
type Client struct {
	endpoint   *url.URL
	flushLimit int
	flushLock  *sync.Mutex
	payload    payloadJSON
}

// NewClient returns a Client configured to send logs to the LogDNA ingest API.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("APIKey missing in Config")
	}

	if cfg.Hostname == "" {
		// try using host name reported by kernel instead
		h, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		if h == "" {
			return nil, fmt.Errorf("Hostname missing in Config and could not use os.Hostname")
		}

		cfg.Hostname = h
	}

	if cfg.FlushLimit == 0 {
		cfg.FlushLimit = DefaultFlushLimit
	}

	endpoint, err := makeEndpoint(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{
		endpoint:   endpoint,
		flushLimit: cfg.FlushLimit,
		flushLock:  &sync.Mutex{},
	}, nil
}

// makeEndpoint creates a new URL to the full LogDNA ingest API endpoint with
// API key and hostname parameters.
func makeEndpoint(cfg Config) (*url.URL, error) {
	u, err := url.Parse(IngestBaseURL)
	if err != nil {
		return nil, err
	}

	u.User = url.User(cfg.APIKey)
	values := url.Values{}
	values.Set("hostname", cfg.Hostname)
	// TODO: handle more parameters
	u.RawQuery = values.Encode()

	return u, err
}

// nowToMs returns milliseconds for a given Time
//
// Ingest API wants timestamp in milliseconds so we need to round timestamp
// down from nanoseconds.
func nowToMs(t time.Time) int64 {
	return t.UnixNano() / 1e6
}

// refreshEndpoint updates the `now` parameter for the ingest API endpoint
func (c *Client) refreshEndpoint() string {
	q := c.endpoint.Query()
	t := time.Now()
	m := nowToMs(t)
	q.Set("now", strconv.FormatInt(m, 10))
	c.endpoint.RawQuery = q.Encode()

	return c.endpoint.String()
}

// Log adds a new log line to Client's payload.
//
// To actually send the logs, Flush() needs to be called.
//
// Flush is called automatically if we reach the client's flush limit.
func (c *Client) Log(t time.Time, msg string) {
	c.flushLock.Lock()
	c.payload.Lines = append(c.payload.Lines, logLineJSON{
		Timestamp: nowToMs(t),
		Line:      msg,
		// TODO: handle more attributes
	})
	c.flushLock.Unlock()

	if c.Size() >= c.flushLimit {
		c.Flush()
	}
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

	//prevent concurrent Flush()es from stepping on one another
	c.flushLock.Lock()
	defer c.flushLock.Unlock()

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(c.payload); err != nil {
		return err
	}

	resp, err := http.Post(c.refreshEndpoint(), "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		c.payload = payloadJSON{}
		return nil
	default:
		// TODO: handle known error cases better
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf(string(b))
	}
}

// Close closes the client. It also sends any buffered logs.
func (c *Client) Close() error {
	return c.Flush()
}
