// A trivial LightStep Go tracer example.
//
// $ go build -o lightstep_trivial github.com/lightstep/lightstep-tracer-go/examples/trivial
// $ ./lightstep_trivial --access_token=YOUR_ACCESS_TOKEN

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	logger "log"
	"os"
	"reflect"
	"time"

	lightstep "github.com/lightstep/lightstep-tracer-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

var accessToken = flag.String("access_token", "", "your LightStep access token")

func subRoutine(ctx context.Context) {
	trivialSpan, _ := opentracing.StartSpanFromContext(ctx, "test span")
	defer trivialSpan.Finish()
	trivialSpan.LogEvent("logged something")
	trivialSpan.LogFields(log.String("string_key", "some string value"), log.Object("trivialSpan", trivialSpan))

	subSpan := opentracing.StartSpan(
		"child span", opentracing.ChildOf(trivialSpan.Context()))
	trivialSpan.LogFields(log.Int("int_key", 42), log.Object("subSpan", subSpan),
		log.String("time.eager", fmt.Sprint(time.Now())),
		log.Lazy(func(fv log.Encoder) {
			fv.EmitString("time.lazy", fmt.Sprint(time.Now()))
		}))
	defer subSpan.Finish()
}

func asyncRoutine(ctx context.Context) {
	span, _ := opentracing.StartSpanFromContext(ctx, "test async goroutine")
	type datatum struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	data := datatum{Name: "test", Value: 100}
	d, _ := json.Marshal(data)
	span.LogEvent("logged inside async")
	span.LogKV("jsondata eager", string(d))
	span.LogFields(log.Object("json.eager.object", d))
	span.LogFields(log.Lazy(func(fv log.Encoder) {
		fv.EmitString("json.lazy.string", string(d))
	}))
	span.LogFields(log.Lazy(func(fv log.Encoder) {
		fv.EmitString("json.lazy.fmt.Sprint", fmt.Sprint(d))
	}))
	span.LogFields(log.Lazy(func(fv log.Encoder) {
		fv.EmitString("json.lazy.fmt.Sprintf", fmt.Sprintf("%#v", d))
	}))
	defer span.Finish()
}

type LoggingRecorder struct {
	r lightstep.SpanRecorder
}

func (r *LoggingRecorder) RecordSpan(span lightstep.RawSpan) {
	logger.Printf("span traceID: %v spanID: %v parentID: %v Operation: %v \n", span.Context.TraceID, span.Context.SpanID, span.ParentSpanID, span.Operation)
}

func main() {
	flag.Parse()
	if len(*accessToken) == 0 {
		fmt.Println("You must specify --access_token")
		os.Exit(1)
	}

	lightstep.SetGlobalEventHandler(logTracerEventHandler)

	loggableRecorder := &LoggingRecorder{}

	// Use LightStep as the global OpenTracing Tracer.
	opentracing.InitGlobalTracer(lightstep.NewTracer(lightstep.Options{
		AccessToken: *accessToken,
		Collector:   lightstep.Endpoint{Host: "collector.lightstep.com", Plaintext: true},
		UseHttp:     true,
		Recorder:    loggableRecorder,
	}))

	// Do something that's traced.
	subRoutine(context.Background())

	// Async
	asyncRoutine(context.Background())

	// Force a flush before exit.
	err := lightstep.FlushLightStepTracer(opentracing.GlobalTracer())
	if err != nil {
		panic(err)
	}
}

func logTracerEventHandler(event lightstep.Event) {
	switch event := event.(type) {
	case lightstep.EventStatusReport:
		logger.Printf("LightStep status report status %s", event.String())
	case lightstep.EventConnectionError:
		logger.Printf("LightStep connection error %s", event.Err())
	case lightstep.EventStartError:
		logger.Printf("LightStep start error %s", event.Err())
	case lightstep.ErrorEvent:
		logger.Printf("LightStep error %s", event.Err())
	default:
		logger.Printf("LightStep unknown event event %s type %s", event.String(), reflect.TypeOf(event))
	}
}
