package items

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/your-org/go-react-starter/backend/internal/apiutil"
	"github.com/your-org/go-react-starter/backend/internal/audit"
	"github.com/your-org/go-react-starter/backend/internal/rbac"
)

type Handler struct {
	repo  *Repo
	audit audit.Recorder
}

func NewHandler(repo *Repo, rec audit.Recorder) *Handler { return &Handler{repo: repo, audit: rec} }

var bearer = []map[string][]string{{"bearer": {}}}

func (h *Handler) Register(api huma.API) {
	tag := []string{"Items"}
	huma.Register(api, huma.Operation{OperationID: "item-create", Method: http.MethodPost, Path: "/items",
		Summary: "Create an item", Tags: tag, Security: bearer, DefaultStatus: http.StatusCreated}, h.create)
	huma.Register(api, huma.Operation{OperationID: "item-list", Method: http.MethodGet, Path: "/items",
		Summary: "List items in your org", Tags: tag, Security: bearer}, h.list)
	huma.Register(api, huma.Operation{OperationID: "item-get", Method: http.MethodGet, Path: "/items/{id}",
		Summary: "Get an item", Tags: tag, Security: bearer}, h.get)
	huma.Register(api, huma.Operation{OperationID: "item-update", Method: http.MethodPatch, Path: "/items/{id}",
		Summary: "Update an item (admin only)", Tags: tag, Security: bearer}, h.update)
	huma.Register(api, huma.Operation{OperationID: "item-delete", Method: http.MethodDelete, Path: "/items/{id}",
		Summary: "Delete an item (admin only)", Tags: tag, Security: bearer, DefaultStatus: http.StatusNoContent}, h.delete)
}

type itemBody struct {
	ID          string  `json:"id"`
	OrgID       string  `json:"org_id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

func toBody(it *Item) itemBody {
	return itemBody{ID: it.ID.String(), OrgID: it.OrgID.String(), Name: it.Name, Description: it.Description}
}

type createBody struct {
	Name        string  `json:"name" minLength:"1"`
	Description *string `json:"description,omitempty"`
}
type createInput struct{ Body createBody }
type itemOutput struct{ Body itemBody }
type listOutput struct {
	Body struct {
		Items []itemBody `json:"items"`
	}
}
type idPath struct{ ID uuid.UUID `path:"id"` }
type updateBody struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}
type updateInput struct {
	ID   uuid.UUID `path:"id"`
	Body updateBody
}

func (h *Handler) create(ctx context.Context, in *createInput) (*itemOutput, error) {
	if err := apiutil.RequireAnyRole(ctx, rbac.RoleAdmin, rbac.RoleUser); err != nil {
		return nil, err
	}
	orgID, err := apiutil.ResolveOrg(ctx, nil)
	if err != nil {
		return nil, err
	}
	actor := apiutil.Identity(ctx).UserID
	it, err := h.repo.Create(ctx, NewItem{OrgID: orgID, Name: in.Body.Name, Description: in.Body.Description, CreatedBy: &actor})
	if err != nil {
		return nil, huma.Error500InternalServerError("could not create item")
	}
	apiutil.Audit(ctx, h.audit, "item.create", "item", it.ID.String())
	return &itemOutput{Body: toBody(it)}, nil
}

func (h *Handler) list(ctx context.Context, _ *struct{}) (*listOutput, error) {
	if err := apiutil.RequireAnyRole(ctx, rbac.RoleAdmin, rbac.RoleUser); err != nil {
		return nil, err
	}
	orgFilter := apiutil.Identity(ctx).EffectiveOrgFilter()
	its, err := h.repo.List(ctx, orgFilter)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list items")
	}
	out := &listOutput{}
	for i := range its {
		out.Body.Items = append(out.Body.Items, toBody(&its[i]))
	}
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *idPath) (*itemOutput, error) {
	if err := apiutil.RequireAnyRole(ctx, rbac.RoleAdmin, rbac.RoleUser); err != nil {
		return nil, err
	}
	it, err := h.repo.GetByID(ctx, in.ID, apiutil.Identity(ctx).EffectiveOrgFilter())
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("item not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not load item")
	}
	apiutil.Audit(ctx, h.audit, "item.read", "item", it.ID.String())
	return &itemOutput{Body: toBody(it)}, nil
}

func (h *Handler) update(ctx context.Context, in *updateInput) (*itemOutput, error) {
	if err := apiutil.RequireAnyRole(ctx, rbac.RoleAdmin); err != nil {
		return nil, err
	}
	it, err := h.repo.Update(ctx, in.ID, apiutil.Identity(ctx).EffectiveOrgFilter(), Patch{Name: in.Body.Name, Description: in.Body.Description})
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("item not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not update item")
	}
	apiutil.Audit(ctx, h.audit, "item.update", "item", it.ID.String())
	return &itemOutput{Body: toBody(it)}, nil
}

func (h *Handler) delete(ctx context.Context, in *idPath) (*struct{}, error) {
	if err := apiutil.RequireAnyRole(ctx, rbac.RoleAdmin); err != nil {
		return nil, err
	}
	err := h.repo.Delete(ctx, in.ID, apiutil.Identity(ctx).EffectiveOrgFilter())
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("item not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not delete item")
	}
	apiutil.Audit(ctx, h.audit, "item.delete", "item", in.ID.String())
	return nil, nil
}
