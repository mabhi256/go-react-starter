// Package queueadmin exposes super-admin endpoints for inspecting and recovering
// background task queues. "Archived" tasks are those that exhausted all retries
// (the DLQ); they can be re-enqueued or purged from here.
package queueadmin

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hibiken/asynq"

	"github.com/your-org/go-react-starter/backend/internal/apiutil"
	"github.com/your-org/go-react-starter/backend/internal/audit"
)

type Handler struct {
	inspector *asynq.Inspector
	audit     audit.Recorder
}

func NewHandler(inspector *asynq.Inspector, rec audit.Recorder) *Handler {
	return &Handler{inspector: inspector, audit: rec}
}

var bearer = []map[string][]string{{"bearer": {}}}

func (h *Handler) Register(api huma.API) {
	tag := []string{"Queue Admin (super-admin)"}
	huma.Register(api, huma.Operation{
		OperationID: "queue-list", Method: http.MethodGet, Path: "/admin/queues",
		Summary: "List queue stats including archived (DLQ) counts", Tags: tag, Security: bearer,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "queue-requeue-archived", Method: http.MethodPost, Path: "/admin/queues/{queue}/archived/requeue",
		Summary: "Re-enqueue all archived (dead) tasks", Tags: tag, Security: bearer, DefaultStatus: http.StatusNoContent,
	}, h.requeueArchived)
	huma.Register(api, huma.Operation{
		OperationID: "queue-purge-archived", Method: http.MethodDelete, Path: "/admin/queues/{queue}/archived",
		Summary: "Purge all archived (dead) tasks", Tags: tag, Security: bearer, DefaultStatus: http.StatusNoContent,
	}, h.purgeArchived)
}

// ---- I/O ----

type queueStats struct {
	Queue    string `json:"queue"`
	Active   int    `json:"active"`
	Pending  int    `json:"pending"`
	Retry    int    `json:"retry"`
	Archived int    `json:"archived" doc:"Tasks that exhausted all retries (DLQ)"`
}

type listOutput struct {
	Body struct {
		Queues []queueStats `json:"queues"`
	}
}

type queuePath struct {
	Queue string `path:"queue"`
}

// ---- handlers ----

func (h *Handler) list(ctx context.Context, _ *struct{}) (*listOutput, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	queues, err := h.inspector.Queues()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list queues")
	}
	out := &listOutput{}
	for _, q := range queues {
		info, err := h.inspector.GetQueueInfo(q)
		if err != nil {
			continue
		}
		out.Body.Queues = append(out.Body.Queues, queueStats{
			Queue:    q,
			Active:   info.Active,
			Pending:  info.Pending,
			Retry:    info.Retry,
			Archived: info.Archived,
		})
	}
	return out, nil
}

func (h *Handler) requeueArchived(ctx context.Context, in *queuePath) (*struct{}, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := h.inspector.RunAllArchivedTasks(in.Queue); err != nil {
		return nil, huma.Error500InternalServerError("could not requeue archived tasks")
	}
	apiutil.Audit(ctx, h.audit, "queue.archived.requeue", "queue", in.Queue)
	return nil, nil
}

func (h *Handler) purgeArchived(ctx context.Context, in *queuePath) (*struct{}, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := h.inspector.DeleteAllArchivedTasks(in.Queue); err != nil {
		return nil, huma.Error500InternalServerError("could not purge archived tasks")
	}
	apiutil.Audit(ctx, h.audit, "queue.archived.purge", "queue", in.Queue)
	return nil, nil
}

