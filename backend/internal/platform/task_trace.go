package platform

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type taskEnvelope struct {
	TC map[string]string `json:"tc,omitempty"`
	P  json.RawMessage   `json:"p"`
}

// WrapTaskPayload embeds the current trace context alongside the payload so the
// worker can continue the same trace.
func WrapTaskPayload(ctx context.Context, payload []byte) ([]byte, error) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return json.Marshal(taskEnvelope{TC: carrier, P: payload})
}

// UnwrapTaskPayload extracts the trace context from an envelope and returns the
// inner payload. If data is not an envelope it is returned unchanged.
func UnwrapTaskPayload(ctx context.Context, data []byte) (context.Context, []byte) {
	var e taskEnvelope
	if err := json.Unmarshal(data, &e); err != nil || e.P == nil {
		return ctx, data
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(e.TC))
	return ctx, []byte(e.P)
}
