package nrlambda

// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.

// This file contains code copied from github.com/aws/aws-lambda-go released
// under the license: https://github.com/aws/aws-lambda-go/blob/master/LICENSE

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aws/aws-lambda-go/lambda"
)

// lambdaHandler is the generic function type
type lambdaHandler func(context.Context, []byte) (interface{}, error)

// Invoke calls the handler, and serializes the response.
// If the underlying handler returned an error, or an error occurs during serialization, error is returned.
func (handler lambdaHandler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	response, err := handler(ctx, payload)
	if err != nil {
		return nil, err
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return responseBytes, nil
}

func errorHandler(e error) lambdaHandler {
	return func(ctx context.Context, event []byte) (interface{}, error) {
		return nil, e
	}
}

func validateArguments(handler reflect.Type) (bool, error) {
	handlerTakesContext := false
	if handler.NumIn() > 2 {
		return false, fmt.Errorf("handlers may not take more than two arguments, but handler takes %d", handler.NumIn())
	} else if handler.NumIn() > 0 {
		contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
		argumentType := handler.In(0)
		handlerTakesContext = argumentType.Implements(contextType)
		if handler.NumIn() > 1 && !handlerTakesContext {
			return false, fmt.Errorf("handler takes two arguments, but the first is not Context. got %s", argumentType.Kind())
		}
	}

	return handlerTakesContext, nil
}

func validateReturns(handler reflect.Type) error {
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if handler.NumOut() > 2 {
		return fmt.Errorf("handler may not return more than two values")
	} else if handler.NumOut() > 1 {
		if !handler.Out(1).Implements(errorType) {
			return fmt.Errorf("handler returns two values, but the second does not implement error")
		}
	} else if handler.NumOut() == 1 {
		if !handler.Out(0).Implements(errorType) {
			return fmt.Errorf("handler returns a single value, but it does not implement error")
		}
	}
	return nil
}

// HandlerTrace allows handlers which wrap the return value of NewHandler to
// access to the request and response events.
type HandlerTrace struct {
	RequestEvent  func(context.Context, interface{})
	ResponseEvent func(context.Context, interface{})
}

func callbackCompose(f1, f2 func(context.Context, interface{})) func(context.Context, interface{}) {
	return func(ctx context.Context, event interface{}) {
		if nil != f1 {
			f1(ctx, event)
		}
		if nil != f2 {
			f2(ctx, event)
		}
	}
}

type handlerTraceKey struct{}

// WithHandlerTrace adds callbacks to the provided context which allows handlers
// which wrap the return value of NewHandler to access to the request and
// response events.
func withHandlerTrace(ctx context.Context, trace HandlerTrace) context.Context {
	existing := contextHandlerTrace(ctx)
	return context.WithValue(ctx, handlerTraceKey{}, HandlerTrace{
		RequestEvent:  callbackCompose(existing.RequestEvent, trace.RequestEvent),
		ResponseEvent: callbackCompose(existing.ResponseEvent, trace.ResponseEvent),
	})
}

func contextHandlerTrace(ctx context.Context) HandlerTrace {
	trace, _ := ctx.Value(handlerTraceKey{}).(HandlerTrace)
	return trace
}

// NewHandler creates a base lambda handler from the given handler function. The
// returned Handler performs JSON deserialization and deserialization, and
// delegates to the input handler function.  The handler function parameter must
// satisfy the rules documented by Start.  If handlerFunc is not a valid
// handler, the returned Handler simply reports the validation error.
func newHandler(handlerFunc interface{}) lambda.Handler {
	if handlerFunc == nil {
		return errorHandler(fmt.Errorf("handler is nil"))
	}
	handler := reflect.ValueOf(handlerFunc)
	handlerType := reflect.TypeOf(handlerFunc)
	if handlerType.Kind() != reflect.Func {
		return errorHandler(fmt.Errorf("handler kind %s is not %s", handlerType.Kind(), reflect.Func))
	}

	takesContext, err := validateArguments(handlerType)
	if err != nil {
		return errorHandler(err)
	}

	if err := validateReturns(handlerType); err != nil {
		return errorHandler(err)
	}

	return lambdaHandler(func(ctx context.Context, payload []byte) (interface{}, error) {

		trace := contextHandlerTrace(ctx)

		// construct arguments
		var args []reflect.Value
		if takesContext {
			args = append(args, reflect.ValueOf(ctx))
		}
		if (handlerType.NumIn() == 1 && !takesContext) || handlerType.NumIn() == 2 {
			eventType := handlerType.In(handlerType.NumIn() - 1)
			event := reflect.New(eventType)

			if err := json.Unmarshal(payload, event.Interface()); err != nil {
				return nil, err
			}
			if nil != trace.RequestEvent {
				trace.RequestEvent(ctx, event.Elem().Interface())
			}
			args = append(args, event.Elem())
		}

		response := handler.Call(args)

		// convert return values into (interface{}, error)
		var err error
		if len(response) > 0 {
			if errVal, ok := response[len(response)-1].Interface().(error); ok {
				err = errVal
			}
		}
		var val interface{}
		if len(response) > 1 {
			val = response[0].Interface()

			if nil != trace.ResponseEvent {
				trace.ResponseEvent(ctx, val)
			}
		}

		return val, err
	})
}
