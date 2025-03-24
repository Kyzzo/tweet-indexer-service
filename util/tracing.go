package util

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func ExtractTraceParentFromKafkaMessage(msg kafka.Message) string {
	for _, h := range msg.Headers {
		if h.Key == "traceparent" {
			return string(h.Value)
		}
	}
	return ""
}

func CreateTracer() oteltrace.Tracer {
	tp := otel.GetTracerProvider()
	tracer := tp.Tracer(
		otelgin.ScopeName,
		oteltrace.WithInstrumentationVersion(otelgin.Version()),
	)
	return tracer
}

func CreateContextFromTraceParent(ctx context.Context, traceparent string) context.Context {
	if traceparent != "" {
		propagator := propagation.TraceContext{}
		carrier := propagation.MapCarrier{
			"traceparent": traceparent,
		}

		ctx = propagator.Extract(ctx, carrier)
	}

	return ctx
}

func CreateBackgroundContextFromExisting(ctx context.Context) context.Context {
	currentSpan := oteltrace.SpanFromContext(ctx)

	traceID := currentSpan.SpanContext().TraceID()
	spanID := currentSpan.SpanContext().SpanID()

	newSpanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	newCtx := context.Background()
	newCtx = oteltrace.ContextWithRemoteSpanContext(newCtx, newSpanContext)

	return newCtx
}
