package nrlambda

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	newrelic "github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/internal"
)

func testApp(t *testing.T) newrelic.Application {
	cfg := newrelic.NewConfig("", "")
	cfg.Enabled = false
	cfg.ServerlessMode.Enabled = true
	app, err := newrelic.NewApplication(cfg)
	if nil != err {
		t.Fatal(err)
	}
	internal.HarvestTesting(app, nil)
	return app
}

func TestColdStart(t *testing.T) {
	originalHandler := func(c context.Context) {}
	app := testApp(t)
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
	app := testApp(t)
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
	app := testApp(t)
	wrapped := Wrap(originalHandler, app)
	w := wrapped.(*wrappedHandler)
	w.functionName = "functionName"

	resp, err := wrapped.Invoke(context.Background(), []byte(`{}`))
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
