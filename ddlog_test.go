package ddlog_test

import (
	"testing"

	ddlog "github.com/twistingmercury/go-datadog"
)

func getConfig() ddlog.DDConfig {
	return ddlog.DDConfig{
		TraceIntakeHost: "",
		TraceIntakePort: 9456,
		LogIntakeURL:    "",
		LogIntakePort:   6789,
		APIKey:          "",
		Environment:     "unit test",
		ServiceName:     "",
		ServiceVersion:  "",
		GlobalTags:      make(map[string]interface{}, 0),
		Commit:          "",
		LogBufferSize:   1,
		LogTimeout:      1,
	}
}

func TestInitialize(t *testing.T) {

}
