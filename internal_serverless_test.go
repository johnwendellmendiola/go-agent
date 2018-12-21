package newrelic

import (
	"encoding/json"
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
		cfg.ServerlessMode.TrustKey = "trustkey"
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
		cfg.ServerlessMode.TrustKey = "trustkey"
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

func TestServerlessJSON(t *testing.T) {
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
	}
	app := testApp(nil, cfgFn, t)
	txn := app.StartTransaction("hello", nil, nil)
	txn.(internal.AddAgentAttributer).AddAgentAttribute(internal.AttributeAWSLambdaARN, "thearn", nil)
	txn.End()
	js, err := txn.(serverlessTransaction).serverlessPayloadJSON("executionEnv")
	if nil != err {
		t.Error(err)
	}
	var p serverlessPayload
	err = json.Unmarshal(js, &p)
	if nil != err {
		t.Error(err)
	}
	// Data should contain txn event and metrics.  Timestamps make exact
	// JSON comparison tough.
	if len(p.Data) != 2 {
		t.Error(p.Data)
	}
	if p.Metadata.ARN != "thearn" {
		t.Error(p.Metadata.ARN)
	}
	if p.Metadata.AgentVersion != Version {
		t.Error(p.Metadata.AgentVersion)
	}
	if p.Metadata.ExecutionEnvironment != "executionEnv" {
		t.Error(p.Metadata.ExecutionEnvironment)
	}
	if p.Metadata.ProtocolVersion != internal.ProcotolVersion {
		t.Error(p.Metadata.ProtocolVersion)
	}
	if p.Metadata.AgentLanguage != "go" {
		t.Error(p.Metadata.AgentLanguage)
	}
	if p.Metadata.MetadataVersion != 2 {
		t.Error(p.Metadata.MetadataVersion)
	}
}

func TestServerlessJSONMissingARN(t *testing.T) {
	// serverlessPayloadJSON should not panic if the Lambda ARN is missing.
	cfgFn := func(cfg *Config) {
		cfg.ServerlessMode.Enabled = true
	}
	app := testApp(nil, cfgFn, t)
	txn := app.StartTransaction("hello", nil, nil)
	txn.End()
	js, err := txn.(serverlessTransaction).serverlessPayloadJSON("executionEnv")
	if nil != err {
		t.Error(err)
	}
	if nil == js {
		t.Error("missing JSON")
	}
}
