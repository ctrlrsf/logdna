package logdna

import (
	"encoding/json"
	"testing"
	"time"
)

var testConfig = Config{
	APIKey:   "secret",
	LogFile:  "test1.log",
	Hostname: "testhost.com",
}

func TestPayloadJSONMarshaling(t *testing.T) {
	logLine1 := logLineJSON{
		Timestamp: 1469047048,
		Line:      "Test line 1",
		File:      "test.log",
	}
	logLine2 := logLineJSON{
		Timestamp: 1469146012,
		Line:      "Test line 2",
		File:      "test.log",
	}

	logLines := []logLineJSON{logLine1, logLine2}

	payload := payloadJSON{
		Lines: logLines,
	}
	t.Logf("PayloadJSON value: %+v", payload)
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("unable marshal payload to JSON: %v", err)
	}

	t.Logf("PayloadJSON as JSON string: %s", jsonPayload)
}

func TestClient_Log(t *testing.T) {
	client := NewClient(testConfig)

	logMsg := "Test log message"
	client.Log(time.Time{}, logMsg, "Info")

	if client.payload.Lines[0].Line != logMsg {
		t.Fatalf("did not add expected log line")
	}
}

func TestClient_Size(t *testing.T) {
	client := NewClient(testConfig)

	logMsg := "Test log message"
	client.Log(time.Time{}, logMsg, "Info")

	if client.Size() != 1 {
		t.Fatalf("size is wrong: expected 1 got %d", client.Size())
	}
}
