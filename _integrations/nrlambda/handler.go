// Package nrlambda adds support for AWS Lambda.
//
// Example: https://github.com/newrelic/go-agent/tree/master/_integrations/nrlambda/example/main.go
package nrlambda

import (
	"context"
	"sync"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	newrelic "github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/internal"
)

func (h *wrappedHandler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	txn := h.app.StartTransaction(h.functionName, nil, nil)
	defer txn.End()

	if aa, ok := txn.(internal.AddAgentAttributer); ok {
		if lctx, ok := lambdacontext.FromContext(ctx); ok {
			aa.AddAgentAttribute(internal.AttributeAWSRequestID, lctx.AwsRequestID, nil)
			aa.AddAgentAttribute(internal.AttributeAWSLambdaARN, lctx.InvokedFunctionArn, nil)
		}

		h.firstTransaction.Do(func() {
			aa.AddAgentAttribute(internal.AttributeAWSLambdaColdStart, "", true)
		})
	}

	ctx = newrelic.NewContext(ctx, txn)

	response, err := h.original.Invoke(ctx, payload)

	if nil != err {
		txn.NoticeError(err)
	}

	return response, err
}

type wrappedHandler struct {
	original lambda.Handler
	app      newrelic.Application
	// functionName is copied from lambdacontext.FunctionName for
	// deterministic tests that don't depend on environment variables.
	functionName string
	// Although we are told that each Lambda will only handle one request at
	// a time, we use a synchronization primitive to determine if this is
	// the first transaction for defensiveness in case of future changes.
	firstTransaction sync.Once
}

// WrapHandler wraps the provided handler and returns a new handler with
// instrumentation. StartHandler should generally be used in place of
// WrapHandler: this function is exposed for consumers who are chaining
// middlewares.
func WrapHandler(handler lambda.Handler, app newrelic.Application) lambda.Handler {
	if nil == app {
		return handler
	}
	return &wrappedHandler{
		original:     handler,
		app:          app,
		functionName: lambdacontext.FunctionName,
	}
}

// Wrap wraps the provided handler and returns a new handler with
// instrumentation. Start should generally be used in place of Wrap: this
// function is exposed for consumers who are chaining middlewares.
func Wrap(handler interface{}, app newrelic.Application) lambda.Handler {
	return WrapHandler(lambda.NewHandler(handler), app)
}

// Start should be used in place of lambda.Start.
//
// lambda.Start(myhandler) => nrlambda.Start(myhandler, app)
//
func Start(handler interface{}, app newrelic.Application) {
	lambda.StartHandler(Wrap(handler, app))
}

// StartHandler should be used in place of lambda.StartHandler.
//
// lambda.StartHandler(myhandler) => nrlambda.StartHandler(myhandler, app)
//
func StartHandler(handler lambda.Handler, app newrelic.Application) {
	lambda.StartHandler(WrapHandler(handler, app))
}
