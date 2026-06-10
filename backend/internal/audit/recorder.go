package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"

	"github.com/your-org/go-react-starter/backend/internal/platform"
)

// TaskWrite is the asynq task type for appending an audit entry.
const TaskWrite = "audit:write"

// Recorder records an audit entry. Implementations may be sync or async.
type Recorder interface {
	Record(ctx context.Context, e Entry) error
}

// AsynqRecorder enqueues entries onto the audit queue for the worker to persist.
type AsynqRecorder struct {
	client *asynq.Client
}

func NewAsynqRecorder(client *asynq.Client) *AsynqRecorder { return &AsynqRecorder{client: client} }

func (r *AsynqRecorder) Record(ctx context.Context, e Entry) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	wrapped, err := platform.WrapTaskPayload(ctx, payload)
	if err != nil {
		return fmt.Errorf("wrap task payload: %w", err)
	}
	_, span := otel.Tracer("audit").Start(ctx, "asynq.enqueue")
	_, err = r.client.EnqueueContext(ctx, asynq.NewTask(TaskWrite, wrapped), asynq.Queue("audit"), asynq.MaxRetry(5))
	span.End()
	return err
}

// HandleWrite returns the asynq handler that persists enqueued audit entries.
func HandleWrite(repo *Repo, logger zerolog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var e Entry
		if err := json.Unmarshal(t.Payload(), &e); err != nil {
			// Bad payload will never succeed on retry; log and drop via SkipRetry.
			logger.Error().Err(err).Msg("audit: bad task payload")
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return repo.Insert(ctx, e)
	}
}

