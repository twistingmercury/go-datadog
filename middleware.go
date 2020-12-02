package ddlog

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/sirupsen/logrus"
)

var (
	appVer  string
	appName string
)

// Monitor is middleware that logs and traces the calls.
func Monitor() gin.HandlerFunc {
	return func(c *gin.Context) {
		opts := []ddtrace.StartSpanOption{
			tracer.SpanType(ext.SpanTypeWeb),
			tracer.Tag(ext.HTTPMethod, c.Request.Method),
			tracer.Tag(ext.HTTPURL, c.Request.URL.Path),
			tracer.Measured(),
		}
		span, _ := tracer.StartSpanFromContext(c.Request.Context(), c.Request.RequestURI, opts...)
		defer func(sc ddtrace.Span, started time.Time) {
			sc.SetTag(ext.HTTPCode, strconv.Itoa(c.Writer.Status()))
			l := logEntry(sc, c.Request, started, c.Writer.Status())
			if err := recover(); err != nil {
				l.Error("error", err)
				sc.Finish(tracer.WithError(err.(error)))
			} else {
				l.Info("success")
				sc.Finish()
			}
		}(span, time.Now())
		c.Next()
	}
}

func logEntry(sc ddtrace.Span, c *http.Request, s time.Time, rc int) *logrus.Entry {
	tid := sc.Context().TraceID()
	sid := sc.Context().SpanID()
	return logrus.WithFields(
		logrus.Fields{
			"client_ip":        c.RemoteAddr,
			"dd.trace_id":      tid,
			"dd.span_id":       sid,
			"env":              ddCfg.Environment,
			"service":          ddCfg.ServiceName,
			"version":          ddCfg.ServiceVersion,
			"host":             hostName,
			"commit_hash":      ddCfg.Commit,
			"http_method":      c.Method,
			"http_request_url": c.URL,
			"http_status":      strconv.Itoa(rc),
			"user_agent":       c.UserAgent(),
			"elapsed_ms":       float64(time.Since(s).Microseconds()) / 1000,
		})
}
