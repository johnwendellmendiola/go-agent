package nrlambda

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	newrelic "github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/internal"
)

func testApp(cfgfn func(*newrelic.Config), t *testing.T) newrelic.Application {
	cfg := newrelic.NewConfig("", "")
	cfg.Enabled = false
	cfg.ServerlessMode.Enabled = true

	if nil != cfgfn {
		cfgfn(&cfg)
	}

	app, err := newrelic.NewApplication(cfg)
	if nil != err {
		t.Fatal(err)
	}
	internal.HarvestTesting(app, nil)
	return app
}

func distributedTracingEnabled(config *newrelic.Config) {
	config.CrossApplicationTracer.Enabled = false
	config.DistributedTracer.Enabled = true
	config.ServerlessMode.AccountID = "1"
	config.ServerlessMode.TrustKey = "1"
	config.ServerlessMode.PrimaryAppID = "1"
}

func attributesIncludeResponse(config *newrelic.Config) {
	config.Attributes.Include = []string{"response.*"}
}

func TestColdStart(t *testing.T) {
	originalHandler := func(c context.Context) {}
	app := testApp(nil, t)
	wrapped := Wrap(originalHandler, app)
	w := wrapped.(*wrappedHandler)
	w.functionName = "functionName"

	ctx := context.Background()

	resp, err := wrapped.Invoke(ctx, nil)
	if nil != err || string(resp) != "null" {
		t.Error("unexpected response", err, string(resp))
	}
	app.(internal.Expect).ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics:     map[string]interface{}{"name": "OtherTransaction/Go/functionName"},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"aws.lambda.coldStart": true,
		},
	}})
	// Invoke the handler again to test the cold-start attribute absence.
	internal.HarvestTesting(app, nil)
	resp, err = wrapped.Invoke(ctx, nil)
	if nil != err || string(resp) != "null" {
		t.Error("unexpected response", err, string(resp))
	}
	app.(internal.Expect).ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics:      map[string]interface{}{"name": "OtherTransaction/Go/functionName"},
		UserAttributes:  map[string]interface{}{},
		AgentAttributes: map[string]interface{}{},
	}})
}

func TestErrorCapture(t *testing.T) {
	returnError := errors.New("problem")
	originalHandler := func() error { return returnError }
	app := testApp(nil, t)
	wrapped := Wrap(originalHandler, app)
	w := wrapped.(*wrappedHandler)
	w.functionName = "functionName"

	resp, err := wrapped.Invoke(context.Background(), nil)
	if err != returnError || string(resp) != "" {
		t.Error(err, string(resp))
	}
	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{Name: "OtherTransaction/Go/functionName", Scope: "", Forced: true, Data: nil},
		{Name: "OtherTransaction/all", Scope: "", Forced: true, Data: nil},
		// Error metrics test the error capture.
		{Name: "Errors/all", Scope: "", Forced: true, Data: []float64{1, 0, 0, 0, 0, 0}},
		{Name: "Errors/allOther", Scope: "", Forced: true, Data: []float64{1, 0, 0, 0, 0, 0}},
		{Name: "Errors/OtherTransaction/Go/functionName", Scope: "", Forced: true, Data: []float64{1, 0, 0, 0, 0, 0}},
	})
	app.(internal.Expect).ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics:     map[string]interface{}{"name": "OtherTransaction/Go/functionName"},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"aws.lambda.coldStart": true,
		},
	}})
}

func TestWrapNilApp(t *testing.T) {
	originalHandler := func() (int, error) {
		return 123, nil
	}
	wrapped := Wrap(originalHandler, nil)
	ctx := context.Background()
	resp, err := wrapped.Invoke(ctx, nil)
	if nil != err || string(resp) != "123" {
		t.Error("unexpected response", err, string(resp))
	}
}

func TestSetWebRequest(t *testing.T) {
	originalHandler := func(events.APIGatewayProxyRequest) {}
	app := testApp(nil, t)
	wrapped := Wrap(originalHandler, app)
	w := wrapped.(*wrappedHandler)
	w.functionName = "functionName"

	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"X-Forwarded-Port":  "4000",
			"X-Forwarded-Proto": "HTTPS",
		},
	}
	reqbytes, err := json.Marshal(req)
	if err != nil {
		t.Error("unable to marshal json", err)
	}

	resp, err := wrapped.Invoke(context.Background(), reqbytes)
	if err != nil {
		t.Error(err, string(resp))
	}
	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{Name: "Apdex", Scope: "", Forced: true, Data: nil},
		{Name: "Apdex/Go/functionName", Scope: "", Forced: false, Data: nil},
		{Name: "HttpDispatcher", Scope: "", Forced: true, Data: nil},
		{Name: "WebTransaction", Scope: "", Forced: true, Data: nil},
		{Name: "WebTransaction/Go/functionName", Scope: "", Forced: true, Data: nil},
	})
	app.(internal.Expect).ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics: map[string]interface{}{
			"name":             "WebTransaction/Go/functionName",
			"nr.apdexPerfZone": "S",
		},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"aws.lambda.coldStart": true,
		},
	}})
}

func makePayload(app newrelic.Application) string {
	txn := app.StartTransaction("hello", nil, nil)
	return txn.CreateDistributedTracePayload().Text()
}

func TestDistributedTracing(t *testing.T) {
	originalHandler := func(events.APIGatewayProxyRequest) {}
	app := testApp(distributedTracingEnabled, t)
	wrapped := Wrap(originalHandler, app)
	w := wrapped.(*wrappedHandler)
	w.functionName = "functionName"

	req := events.APIGatewayProxyRequest{
		Headers: map[string]string{
			"X-Forwarded-Port":                     "4000",
			"X-Forwarded-Proto":                    "HTTPS",
			newrelic.DistributedTracePayloadHeader: makePayload(app),
		},
	}
	reqbytes, err := json.Marshal(req)
	if err != nil {
		t.Error("unable to marshal json", err)
	}

	resp, err := wrapped.Invoke(context.Background(), reqbytes)
	if err != nil {
		t.Error(err, string(resp))
	}
	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{Name: "Apdex", Scope: "", Forced: true, Data: nil},
		{Name: "Apdex/Go/functionName", Scope: "", Forced: false, Data: nil},
		{Name: "DurationByCaller/App/1/1/HTTPS/all", Scope: "", Forced: false, Data: nil},
		{Name: "DurationByCaller/App/1/1/HTTPS/allWeb", Scope: "", Forced: false, Data: nil},
		{Name: "HttpDispatcher", Scope: "", Forced: true, Data: nil},
		{Name: "Supportability/DistributedTrace/AcceptPayload/Success", Scope: "", Forced: true, Data: nil},
		{Name: "TransportDuration/App/1/1/HTTPS/all", Scope: "", Forced: false, Data: nil},
		{Name: "TransportDuration/App/1/1/HTTPS/allWeb", Scope: "", Forced: false, Data: nil},
		{Name: "WebTransaction", Scope: "", Forced: true, Data: nil},
		{Name: "WebTransaction/Go/functionName", Scope: "", Forced: true, Data: nil},
	})
	app.(internal.Expect).ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics: map[string]interface{}{
			"name":                     "WebTransaction/Go/functionName",
			"nr.apdexPerfZone":         "S",
			"parent.account":           "1",
			"parent.app":               "1",
			"parent.transportType":     "HTTPS",
			"parent.type":              "App",
			"guid":                     internal.MatchAnything,
			"parent.transportDuration": internal.MatchAnything,
			"parentId":                 internal.MatchAnything,
			"parentSpanId":             internal.MatchAnything,
			"priority":                 internal.MatchAnything,
			"sampled":                  internal.MatchAnything,
			"traceId":                  internal.MatchAnything,
		},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"aws.lambda.coldStart": true,
		},
	}})
}

func TestEventARN(t *testing.T) {
	originalHandler := func(events.DynamoDBEvent) {}
	app := testApp(nil, t)
	wrapped := Wrap(originalHandler, app)
	w := wrapped.(*wrappedHandler)
	w.functionName = "functionName"

	req := events.DynamoDBEvent{
		Records: []events.DynamoDBEventRecord{{
			EventSourceArn: "ARN",
		}},
	}

	reqbytes, err := json.Marshal(req)
	if err != nil {
		t.Error("unable to marshal json", err)
	}

	resp, err := wrapped.Invoke(context.Background(), reqbytes)
	if err != nil {
		t.Error(err, string(resp))
	}
	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{Name: "OtherTransaction/all", Scope: "", Forced: true, Data: nil},
		{Name: "OtherTransaction/Go/functionName", Scope: "", Forced: true, Data: nil},
	})
	app.(internal.Expect).ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics: map[string]interface{}{
			"name": "OtherTransaction/Go/functionName",
		},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"aws.lambda.coldStart":       true,
			"aws.lambda.eventSource.arn": "ARN",
		},
	}})
}

func TestAPIGatewayProxyResponse(t *testing.T) {
	originalHandler := func() (events.APIGatewayProxyResponse, error) {
		return events.APIGatewayProxyResponse{
			Body:       "Hello World",
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
		}, nil
	}

	app := testApp(attributesIncludeResponse, t)
	wrapped := Wrap(originalHandler, app)
	w := wrapped.(*wrappedHandler)
	w.functionName = "functionName"

	resp, err := wrapped.Invoke(context.Background(), nil)
	if nil != err {
		t.Error("unexpected err", err)
	}
	if !strings.Contains(string(resp), "Hello World") {
		t.Error("unexpected response", string(resp))
	}

	app.(internal.Expect).ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics: map[string]interface{}{
			"name": "OtherTransaction/Go/functionName",
		},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"aws.lambda.coldStart":         true,
			"httpResponseCode":             "200",
			"response.headers.contentType": "text/html",
		},
	}})
}
