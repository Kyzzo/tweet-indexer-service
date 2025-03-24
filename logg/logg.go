package logg

import (
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
	"os"
)

var Log = NewDefaultLog()

func NewDefaultLog() zerolog.Logger {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger = logger.Hook(tracingHook{}, userAddressHook{}, vaultIdHook{})
	return logger
}

type tracingHook struct{}

func (h tracingHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level != zerolog.NoLevel { // skip logs without a level
		ctx := e.GetCtx()
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() { // only add if span is valid
			e.Str("trace_id", span.SpanContext().TraceID().String())
			e.Str("span_id", span.SpanContext().SpanID().String())
		}
	}
}

type userAddressHook struct{}

func (h userAddressHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level != zerolog.NoLevel { // skip logs without a level
		ctx := e.GetCtx()
		if userAddress := ctx.Value("user_address"); userAddress != nil {
			e.Str("user_address", userAddress.(string))
		}
	}
}

type vaultIdHook struct{}

func (h vaultIdHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level != zerolog.NoLevel { // skip logs without a level
		ctx := e.GetCtx()
		if vaultId := ctx.Value("vault_id"); vaultId != nil {
			e.Str("vault_id", vaultId.(string))
		}
	}
}
