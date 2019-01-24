package newrelic

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/newrelic/go-agent/internal"
)

func TestServerlessDistributedTracingConfigPresent(t *testing.T) {
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
		cfg.DistributedTracer.Enabled = true
		cfg.CrossApplicationTracer.Enabled = false
		cfg.ServerlessMode.AccountID = "123"
		cfg.ServerlessMode.TrustedAccountKey = "trustkey"
		cfg.ServerlessMode.PrimaryAppID = "456"
	}
	app := testApp(nil, cfgFn, t)
	payload := app.StartTransaction("hello", nil, nil).CreateDistributedTracePayload()
	txn := app.StartTransaction("hello", nil, nil)
	txn.AcceptDistributedTracePayload(TransportHTTP, payload)
	txn.End()
	app.ExpectMetrics(t, []internal.WantMetric{
		{Name: "OtherTransaction/Go/hello", Scope: "", Forced: true, Data: nil},
		{Name: "OtherTransaction/all", Scope: "", Forced: true, Data: nil},
		{Name: "DurationByCaller/App/123/456/HTTP/all", Scope: "", Forced: false, Data: nil},
		{Name: "DurationByCaller/App/123/456/HTTP/allOther", Scope: "", Forced: false, Data: nil},
		{Name: "TransportDuration/App/123/456/HTTP/all", Scope: "", Forced: false, Data: nil},
		{Name: "TransportDuration/App/123/456/HTTP/allOther", Scope: "", Forced: false, Data: nil},
		{Name: "Supportability/DistributedTrace/AcceptPayload/Success", Scope: "", Forced: true, Data: singleCount},
	})
}

func TestServerlessDistributedTracingConfigPartiallyPresent(t *testing.T) {
	// This tests that if ServerlessMode.PrimaryAppID is unset it should
	// default to "Unknown".
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
		cfg.DistributedTracer.Enabled = true
		cfg.CrossApplicationTracer.Enabled = false
		cfg.ServerlessMode.AccountID = "123"
		cfg.ServerlessMode.TrustedAccountKey = "trustkey"
	}
	app := testApp(nil, cfgFn, t)
	payload := app.StartTransaction("hello", nil, nil).CreateDistributedTracePayload()
	txn := app.StartTransaction("hello", nil, nil)
	txn.AcceptDistributedTracePayload(TransportHTTP, payload)
	txn.End()
	app.ExpectMetrics(t, []internal.WantMetric{
		{Name: "OtherTransaction/Go/hello", Scope: "", Forced: true, Data: nil},
		{Name: "OtherTransaction/all", Scope: "", Forced: true, Data: nil},
		{Name: "DurationByCaller/App/123/Unknown/HTTP/all", Scope: "", Forced: false, Data: nil},
		{Name: "DurationByCaller/App/123/Unknown/HTTP/allOther", Scope: "", Forced: false, Data: nil},
		{Name: "TransportDuration/App/123/Unknown/HTTP/all", Scope: "", Forced: false, Data: nil},
		{Name: "TransportDuration/App/123/Unknown/HTTP/allOther", Scope: "", Forced: false, Data: nil},
		{Name: "Supportability/DistributedTrace/AcceptPayload/Success", Scope: "", Forced: true, Data: singleCount},
	})
}

func TestServerlessDistributedTracingConfigAbsent(t *testing.T) {
	// Test that payloads do not get created when distributed tracing
	// configuration is not present.
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
		cfg.DistributedTracer.Enabled = true
		cfg.CrossApplicationTracer.Enabled = false
	}
	app := testApp(nil, cfgFn, t)
	txn := app.StartTransaction("hello", nil, nil)
	payload := txn.CreateDistributedTracePayload()
	if "" != payload.Text() {
		t.Error(payload.Text())
	}
}

func TestServerlessLowApdex(t *testing.T) {
	apdex := -1 * time.Second
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
		cfg.ServerlessMode.ApdexThreshold = apdex
	}
	app := testApp(nil, cfgFn, t)
	txn := app.StartTransaction("hello", nil, nil)
	txn.SetWebRequest(nil) // only web gets apdex
	txn.End()

	app.ExpectMetrics(t, []internal.WantMetric{
		{Name: "WebTransaction/Go/hello", Scope: "", Forced: true, Data: nil},
		{Name: "WebTransaction", Scope: "", Forced: true, Data: nil},
		{Name: "HttpDispatcher", Scope: "", Forced: true, Data: nil},
		// third apdex field is failed count
		{Name: "Apdex", Scope: "", Forced: true, Data: []float64{0, 0, 1, apdex.Seconds(), apdex.Seconds(), 0}},
		{Name: "Apdex/Go/hello", Scope: "", Forced: false, Data: []float64{0, 0, 1, apdex.Seconds(), apdex.Seconds(), 0}},
	})
}

func TestServerlessHighApdex(t *testing.T) {
	apdex := 1 * time.Hour
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
		cfg.ServerlessMode.ApdexThreshold = apdex
	}
	app := testApp(nil, cfgFn, t)
	txn := app.StartTransaction("hello", nil, nil)
	txn.SetWebRequest(nil) // only web gets apdex
	txn.End()

	app.ExpectMetrics(t, []internal.WantMetric{
		{Name: "WebTransaction/Go/hello", Scope: "", Forced: true, Data: nil},
		{Name: "WebTransaction", Scope: "", Forced: true, Data: nil},
		{Name: "HttpDispatcher", Scope: "", Forced: true, Data: nil},
		// first apdex field is satisfied count
		{Name: "Apdex", Scope: "", Forced: true, Data: []float64{1, 0, 0, apdex.Seconds(), apdex.Seconds(), 0}},
		{Name: "Apdex/Go/hello", Scope: "", Forced: false, Data: []float64{1, 0, 0, apdex.Seconds(), apdex.Seconds(), 0}},
	})
}

func TestServerlessRecordCustomMetric(t *testing.T) {
	cfgFn := func(cfg *Config) { cfg.ServerlessMode.Enabled = true }
	app := testApp(nil, cfgFn, t)
	err := app.RecordCustomMetric("myMetric", 123.0)
	if err != errMetricServerless {
		t.Error(err)
	}
}

func TestServerlessRecordCustomEvent(t *testing.T) {
	cfgFn := func(cfg *Config) { cfg.ServerlessMode.Enabled = true }
	app := testApp(nil, cfgFn, t)
	err := app.RecordCustomEvent("myType", validParams)
	if err != errCustomEventsServerless {
		t.Error(err)
	}
}

func decodeUncompress(input string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(input)
	if nil != err {
		return nil, err
	}

	buf := bytes.NewBuffer(decoded)
	gz, err := gzip.NewReader(buf)
	if nil != err {
		return nil, err
	}
	var out bytes.Buffer
	io.Copy(&out, gz)
	gz.Close()

	return out.Bytes(), nil
}

func TestServerlessJSON(t *testing.T) {
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
	}
	app := testApp(nil, cfgFn, t)
	txn := app.StartTransaction("hello", nil, nil)
	txn.(internal.AddAgentAttributer).AddAgentAttribute(internal.AttributeAWSLambdaARN, "thearn", nil)
	txn.End()
	payloadJSON, err := txn.(serverlessTransaction).serverlessJSON("executionEnv")
	if nil != err {
		t.Fatal(err)
	}
	var payload []interface{}
	err = json.Unmarshal(payloadJSON, &payload)
	if nil != err {
		t.Fatal(err)
	}
	if len(payload) != 4 {
		t.Fatal(payload)
	}
	if v := payload[0].(float64); v != lambdaMetadataVersion {
		t.Fatal(payload[0], lambdaMetadataVersion)
	}
	if v := payload[1].(string); v != "NR_LAMBDA_MONITORING" {
		t.Fatal(payload[1])
	}
	metadataJSON, err := decodeUncompress(payload[2].(string))
	if nil != err {
		t.Fatal(err)
	}
	dataJSON, err := decodeUncompress(payload[3].(string))
	if nil != err {
		t.Fatal(err)
	}
	var data map[string]interface{}
	err = json.Unmarshal(dataJSON, &data)
	if nil != err {
		t.Fatal(err)
	}
	// Data should contain txn event and metrics.  Timestamps make exact
	// JSON comparison tough.
	if _, ok := data["metric_data"]; !ok {
		t.Fatal(data)
	}
	if _, ok := data["analytic_event_data"]; !ok {
		t.Fatal(data)
	}

	var metadata map[string]interface{}
	err = json.Unmarshal(metadataJSON, &metadata)
	if nil != err {
		t.Fatal(err)
	}
	if v, ok := metadata["metadata_version"].(float64); !ok || v != float64(lambdaMetadataVersion) {
		t.Fatal(metadata["metadata_version"])
	}
	if v, ok := metadata["arn"].(string); !ok || v != "thearn" {
		t.Fatal(metadata["arn"])
	}
	if v, ok := metadata["protocol_version"].(float64); !ok || v != float64(internal.ProcotolVersion) {
		t.Fatal(metadata["protocol_version"])
	}
	if v, ok := metadata["execution_environment"].(string); !ok || v != "executionEnv" {
		t.Fatal(metadata["execution_environment"])
	}
	if v, ok := metadata["agent_version"].(string); !ok || v != Version {
		t.Fatal(metadata["agent_version"])
	}
	if v, ok := metadata["agent_language"].(string); !ok || v != agentLanguage {
		t.Fatal(metadata["agent_language"])
	}
}

func TestServerlessJSONMissingARN(t *testing.T) {
	// serverlessPayloadJSON should not panic if the Lambda ARN is missing.
	// The Lambda ARN is not expected to be missing, but to be safe we need
	// to ensure that txn.Attrs.Agent.StringVal won't panic if its not
	// there.
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
	}
	app := testApp(nil, cfgFn, t)
	txn := app.StartTransaction("hello", nil, nil)
	txn.End()
	payloadJSON, err := txn.(serverlessTransaction).serverlessJSON("executionEnv")
	if nil != err {
		t.Fatal(err)
	}
	if nil == payloadJSON {
		t.Error("missing JSON")
	}
}
