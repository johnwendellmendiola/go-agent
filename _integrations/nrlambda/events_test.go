package nrlambda

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	newrelic "github.com/newrelic/go-agent"
)

func TestGetEventAttributes(t *testing.T) {
	testcases := []struct {
		Name  string
		Input interface{}
		Arn   string
	}{
		{Name: "nil", Input: nil, Arn: ""},
		{Name: "SQSEvent empty", Input: events.SQSEvent{}, Arn: ""},
		{Name: "SQSEvent", Input: events.SQSEvent{
			Records: []events.SQSMessage{{
				EventSourceARN: "ARN",
			}},
		}, Arn: "ARN"},
		{Name: "SNSEvent empty", Input: events.SNSEvent{}, Arn: ""},
		{Name: "SNSEvent", Input: events.SNSEvent{
			Records: []events.SNSEventRecord{{
				EventSubscriptionArn: "ARN",
			}},
		}, Arn: "ARN"},
		{Name: "S3Event empty", Input: events.S3Event{}, Arn: ""},
		{Name: "S3Event", Input: events.S3Event{
			Records: []events.S3EventRecord{{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{
						Arn: "ARN",
					},
				},
			}},
		}, Arn: "ARN"},
		{Name: "DynamoDBEvent empty", Input: events.DynamoDBEvent{}, Arn: ""},
		{Name: "DynamoDBEvent", Input: events.DynamoDBEvent{
			Records: []events.DynamoDBEventRecord{{
				EventSourceArn: "ARN",
			}},
		}, Arn: "ARN"},
		{Name: "CodeCommitEvent empty", Input: events.CodeCommitEvent{}, Arn: ""},
		{Name: "CodeCommitEvent", Input: events.CodeCommitEvent{
			Records: []events.CodeCommitRecord{{
				EventSourceARN: "ARN",
			}},
		}, Arn: "ARN"},
		{Name: "KinesisEvent empty", Input: events.KinesisEvent{}, Arn: ""},
		{Name: "KinesisEvent", Input: events.KinesisEvent{
			Records: []events.KinesisEventRecord{{
				EventSourceArn: "ARN",
			}},
		}, Arn: "ARN"},
		{Name: "KinesisFirehoseEvent", Input: events.KinesisFirehoseEvent{
			DeliveryStreamArn: "ARN",
		}, Arn: "ARN"},
	}

	for _, testcase := range testcases {
		arn := getEventSourceARN(testcase.Input)
		if arn != testcase.Arn {
			t.Error(testcase.Name, arn, testcase.Arn)
		}
	}
}

func TestProxyRequestWrapping(t *testing.T) {
	// First test an empty APIGatewayProxyRequest
	req := proxyRequest{}

	if h := req.Header(); len(h) != 0 {
		t.Error(h)
	}
	if u := req.URL().String(); u != "" {
		t.Error(u)
	}
	if m := req.Method(); m != "" {
		t.Error(m)
	}
	if tr := req.Transport(); tr != newrelic.TransportUnknown {
		t.Error(tr)
	}
	// Now test a populated request.
	req.request.Headers = map[string]string{
		"X-Forwarded-Port":  "4000",
		"X-Forwarded-Proto": "HTTPS",
	}
	req.request.HTTPMethod = "GET"
	req.request.Path = "the/path"
	if h := req.Header(); len(h) != 2 {
		t.Error(h)
	}
	if u := req.URL().String(); u != "//:4000/the/path" {
		t.Error(u)
	}
	if m := req.Method(); m != "GET" {
		t.Error(m)
	}
	if tr := req.Transport(); tr != newrelic.TransportHTTPS {
		t.Error(tr)
	}
}
