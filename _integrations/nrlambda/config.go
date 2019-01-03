package nrlambda

import (
	"os"
	"time"

	newrelic "github.com/newrelic/go-agent"
)

// NewConfig populates a newrelic.Config with correct default settings for a
// Lambda serverless environment.
func NewConfig() newrelic.Config {
	return newConfigInternal(os.Getenv)
}

func newConfigInternal(getenv func(string) string) newrelic.Config {
	cfg := newrelic.NewConfig("", "")

	cfg.ServerlessMode.Enabled = true

	cfg.ServerlessMode.AccountID = getenv("NEW_RELIC_ACCOUNT_ID")
	cfg.ServerlessMode.TrustedAccountKey = getenv("NEW_RELIC_TRUST_KEY")
	cfg.ServerlessMode.PrimaryAppID = getenv("NEW_RELIC_PRIMARY_APPLICATION_ID")

	if s := getenv("NEW_RELIC_APDEX_T"); "" != s {
		if apdex, err := time.ParseDuration(s + "s"); nil == err {
			cfg.ServerlessMode.ApdexThreshold = apdex
		}
	}

	return cfg
}
