package ddlog

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/twistingmercury/go-threadsafe"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// DDConfig are the values required to to initialize the use of
// sending logs and traces to a Datadog Agent.
type DDConfig struct {
	TraceIntakeHost string                 `yaml:"traceintakehost"`
	TraceIntakePort int                    `yaml:"traceintakeport"`
	LogIntakeURL    string                 `yaml:"logintakeurl"`
	LogIntakePort   int                    `yaml:"logintakeport"`
	APIKey          string                 `yaml:"apikey"`
	Environment     string                 `yaml:"environment"`
	ServiceName     string                 `yaml:"servicename"`
	ServiceVersion  string                 `yaml:"serviceversion"`
	GlobalTags      map[string]interface{} `yaml:"globaltags"`
	Commit          string                 `yaml:"commit"`
	LogBufferSize   int                    `yaml:"logbuffersize"`
	LogTimeout      int                    `yaml:"logtimeout"`
}

var ddCfg DDConfig
var logTo time.Duration

// Initialize bootstraps the logging and tracing to be sent to a Datadog Agent.
func Initialize(c DDConfig) (err error) {
	ddCfg = c
	logTo, err = time.ParseDuration(fmt.Sprintf("%ds", c.LogTimeout))
	if err != nil {
		return
	}
	agent := net.JoinHostPort(c.TraceIntakeHost, strconv.Itoa(c.TraceIntakePort))

	opts := []tracer.StartOption{tracer.WithAgentAddr(agent),
		tracer.WithEnv(c.Environment),
		tracer.WithServiceName(c.ServiceName),
		tracer.WithServiceVersion(c.ServiceVersion),
	}

	for k, v := range c.GlobalTags {
		opts = append(opts, tracer.WithGlobalTag(k, v))
	}

	t := opentracer.New(opts...)
	opentracing.SetGlobalTracer(t)
	logrus.SetFormatter(&logrus.JSONFormatter{PrettyPrint: true})
	dw := NewAgentWriter(c.LogIntakeURL, c.LogIntakePort, c.APIKey)
	mw := io.MultiWriter(os.Stdout, dw)
	logrus.SetOutput(mw)
	logrus.SetLevel(logrus.DebugLevel)

	return
}

// Stop invokes stops the started tracer.
func Stop() {
	tracer.Stop()
}

func newLogEntry() *logrus.Entry {
	return logrus.WithFields(
		logrus.Fields{
			"dd.env":     ddCfg.Environment,
			"dd.service": ddCfg.ServiceName,
			"dd.version": ddCfg.ServiceVersion,
			"env":        ddCfg.Environment,
			"service":    ddCfg.ServiceName,
			"version":    ddCfg.ServiceVersion,
			"source":     "go",
		})
}

// Debug writes a debug entry into the log.
func Debug(args ...interface{}) {
	newLogEntry().Debug(args...)
}

// Info writes a info entry to the log.
func Info(args ...interface{}) {
	newLogEntry().Info(args...)
}

// Error writes an error entry to the log.
func Error(args ...interface{}) {
	newLogEntry().Error(args...)
}

var (
	addr         string = "https://http-intake.logs.datadoghq.com"
	port         int    = 443
	apiKey       string
	transmitting bool
	buffer       int = 10
	ilog         *logrus.Logger
	events       threadsafe.SafeSlice = threadsafe.NewSlice(0, 0)
)

// Address is the current address where the log entries will be sent.
func Address() string { return addr }

// Port is the port upon which the address is listening.
func Port() int { return port }

// APIKey is the Datadog API key.
func APIKey() string { return apiKey }

// NewAgentWriter returns a new DDAgentWriter
func NewAgentWriter(url string, portNo int, ddApikey string) io.Writer {
	ilog = logrus.New()
	ilog.SetOutput(os.Stdout)
	ilog.SetLevel(logrus.DebugLevel)

	addr = url
	port = portNo
	apiKey = ddApikey
	transmitting = false

	dw := &agentWriter{
		wchan: make(chan bool),
		pchan: make(chan []byte),
	}

	go push(dw.pchan, dw.wchan)
	go pull(dw.wchan)

	return dw
}

type agentWriter struct {
	wchan chan bool
	pchan chan []byte
}

func (w *agentWriter) Write(entry []byte) (n int, err error) {
	w.pchan <- entry
	n = len(entry)
	return
}

func push(pchan chan []byte, wchan chan bool) {
	for {
		select {
		case e := <-pchan:
			events.Add(e)
			if events.Size() >= ddCfg.LogBufferSize && !isTransmitting() {
				wchan <- true
			}
		}
	}
}

func isTransmitting() (b bool) {
	b = transmitting
	return
}

func setTransmitting(b bool) {
	transmitting = b
	return
}

func pull(wchan chan bool) {
	for {
		select {
		case <-wchan:
			sendEvents()
		}
	}
}

func sendEvents() {
	setTransmitting(true)
	defer func() {
		defer func() {
			if err := recover(); err != nil {
				ilog.Error("panic occurred:", err)
			}
		}()

		setTransmitting(false)
	}()

	for i := 0; i < events.Size(); i++ {
		e := events.Get(i).([]byte)
		if e == nil {
			continue
		}

		addr := fmt.Sprintf("%s:%d/v1/input/%s", addr, port, apiKey)

		client := http.Client{Timeout: logTo}
		r, err := client.Post(
			addr,
			"application/json; charset=utf-8",
			bytes.NewBuffer(e),
		)
		if err != nil {
			ilog.Error(err.Error())
			break
		}

		format := fmt.Sprintf("log ingestion URL: %s; status code: %d; message: %s\n", addr, r.StatusCode, r.Status)
		switch {
		case r.StatusCode >= 400 && r.StatusCode < 500:
			ilog.Errorf("client error: %s", format)
		case r.StatusCode >= 500:
			ilog.Warnf("dd error: %s", format)
		default:
			ilog.Debug("log posted successfully")
			events.Delete(i)
		}
	}
}
