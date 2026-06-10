package user

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"golang.org/x/crypto/bcrypt"

	"github.com/your-org/go-react-starter/backend/internal/apiutil"
	"github.com/your-org/go-react-starter/backend/internal/audit"
	"github.com/your-org/go-react-starter/backend/internal/rbac"
)

// Handler exposes org-scoped user management. Org-admins manage users within their own org;
// super-admins may target any org via the {orgId} path segment.
type Handler struct {
	repo  *Repo
	audit audit.Recorder
}

func NewHandler(repo *Repo, rec audit.Recorder) *Handler { return &Handler{repo: repo, audit: rec} }

var bearer = []map[string][]string{{"bearer": {}}}

func (h *Handler) Register(api huma.API) {
	tag := []string{"Users (org-admin)"}
	huma.Register(api, huma.Operation{OperationID: "user-create", Method: http.MethodPost, Path: "/orgs/{orgId}/users",
		Summary: "Create a user (e.g. doctor)", Tags: tag, Security: bearer, DefaultStatus: http.StatusCreated}, h.create)
	huma.Register(api, huma.Operation{OperationID: "user-list", Method: http.MethodGet, Path: "/orgs/{orgId}/users",
		Summary: "List users in an org", Tags: tag, Security: bearer}, h.list)
	huma.Register(api, huma.Operation{OperationID: "user-get", Method: http.MethodGet, Path: "/orgs/{orgId}/users/{id}",
		Summary: "Get a user", Tags: tag, Security: bearer}, h.get)
	huma.Register(api, huma.Operation{OperationID: "user-update", Method: http.MethodPatch, Path: "/orgs/{orgId}/users/{id}",
		Summary: "Update a user", Tags: tag, Security: bearer}, h.update)
	huma.Register(api, huma.Operation{OperationID: "user-delete", Method: http.MethodDelete, Path: "/orgs/{orgId}/users/{id}",
		Summary: "Soft-delete a user", Tags: tag, Security: bearer, DefaultStatus: http.StatusNoContent}, h.delete)
}

// ---- I/O ----

type userBody struct {
	ID     string   `json:"id"`
	OrgID  *string  `json:"org_id"`
	Name   string   `json:"name"`
	Email  *string  `json:"email"`
	Phone  *string  `json:"phone"`
	Status string   `json:"status"`
	Roles  []string `json:"roles"`
}

func toBody(u *User) userBody {
	b := userBody{ID: u.ID.String(), Name: u.Name, Email: u.Email, Phone: u.Phone, Status: u.Status}
	if u.OrgID != nil {
		s := u.OrgID.String()
		b.OrgID = &s
	}
	for _, r := range u.Roles {
		b.Roles = append(b.Roles, string(r))
	}
	return b
}

type userCreateBody struct {
	Name     string   `json:"name" minLength:"1"`
	Email    *string  `json:"email,omitempty" format:"email"`
	Phone    *string  `json:"phone,omitempty"`
	Password *string  `json:"password,omitempty" minLength:"8" doc:"Required for password login"`
	Roles    []string `json:"roles" doc:"e.g. [\"doctor\"]"`
}
type createInput struct {
	OrgID uuid.UUID `path:"orgId"`
	Body  userCreateBody
}
type userOutput struct{ Body userBody }
type listInput struct {
	OrgID uuid.UUID `path:"orgId"`
}
type userListBody struct {
	Items []userBody `json:"items"`
}
type listOutput struct {
	Body userListBody
}
type itemInput struct {
	OrgID uuid.UUID `path:"orgId"`
	ID    uuid.UUID `path:"id"`
}
type userUpdateBody struct {
	Name   *string `json:"name,omitempty"`
	Status *string `json:"status,omitempty" enum:"active,inactive"`
}
type updateInput struct {
	OrgID uuid.UUID `path:"orgId"`
	ID    uuid.UUID `path:"id"`
	Body  userUpdateBody
}

// ---- handlers ----

func (h *Handler) create(ctx context.Context, in *createInput) (*userOutput, error) {
	orgID, err := h.authzOrg(ctx, in.OrgID)
	if err != nil {
		return nil, err
	}
	roles, err := validateRoles(ctx, in.Body.Roles)
	if err != nil {
		return nil, err
	}

	var hash *string
	if in.Body.Password != nil {
		_, span := otel.Tracer("user").Start(ctx, "bcrypt.hash")
		hashed, err := bcrypt.GenerateFromPassword([]byte(*in.Body.Password), bcrypt.DefaultCost)
		span.End()
		if err != nil {
			return nil, huma.Error500InternalServerError("could not hash password")
		}
		s := string(hashed)
		hash = &s
	}

	u, err := h.repo.CreateWithIdentity(ctx, NewUser{
		OrgID: &orgID, Name: in.Body.Name, Email: in.Body.Email, Phone: in.Body.Phone,
		PasswordHash: hash, Roles: roles,
	}, nil)
	if errors.Is(err, ErrConflict) {
		return nil, huma.Error409Conflict("email or phone already in use")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not create user")
	}
	apiutil.Audit(ctx, h.audit, "user.create", "user", u.ID.String())
	return &userOutput{Body: toBody(u)}, nil
}

func (h *Handler) list(ctx context.Context, in *listInput) (*listOutput, error) {
	orgID, err := h.authzOrg(ctx, in.OrgID)
	if err != nil {
		return nil, err
	}
	users, err := h.repo.List(ctx, orgID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list users")
	}
	out := &listOutput{}
	for i := range users {
		out.Body.Items = append(out.Body.Items, toBody(&users[i]))
	}
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *itemInput) (*userOutput, error) {
	orgID, err := h.authzOrg(ctx, in.OrgID)
	if err != nil {
		return nil, err
	}
	u, err := h.repo.GetByID(ctx, in.ID, &orgID)
	return h.single(u, err)
}

func (h *Handler) update(ctx context.Context, in *updateInput) (*userOutput, error) {
	orgID, err := h.authzOrg(ctx, in.OrgID)
	if err != nil {
		return nil, err
	}
	u, err := h.repo.Update(ctx, in.ID, &orgID, in.Body.Name, in.Body.Status)
	if u != nil {
		apiutil.Audit(ctx, h.audit, "user.update", "user", u.ID.String())
	}
	return h.single(u, err)
}

func (h *Handler) delete(ctx context.Context, in *itemInput) (*struct{}, error) {
	orgID, err := h.authzOrg(ctx, in.OrgID)
	if err != nil {
		return nil, err
	}
	err = h.repo.SoftDelete(ctx, in.ID, &orgID)
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("user not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not delete user")
	}
	apiutil.Audit(ctx, h.audit, "user.delete", "user", in.ID.String())
	return nil, nil
}

// authzOrg ensures the caller is an admin (or super-admin) acting on a permitted org.
func (h *Handler) authzOrg(ctx context.Context, orgID uuid.UUID) (uuid.UUID, error) {
	if err := apiutil.RequireAnyRole(ctx, rbac.RoleAdmin, rbac.RoleSuperAdmin); err != nil {
		return uuid.Nil, err
	}
	return apiutil.ResolveOrg(ctx, &orgID)
}

func (h *Handler) single(u *User, err error) (*userOutput, error) {
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("user not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("user operation failed")
	}
	return &userOutput{Body: toBody(u)}, nil
}

// validateRoles permits admin and user. super_admin can never be granted via this endpoint.
func validateRoles(ctx context.Context, in []string) ([]rbac.Role, error) {
	if len(in) == 0 {
		return nil, huma.Error422UnprocessableEntity("at least one role is required")
	}
	caller := apiutil.Identity(ctx)
	out := make([]rbac.Role, 0, len(in))
	for _, r := range in {
		switch rbac.Role(r) {
		case rbac.RoleUser:
		case rbac.RoleAdmin:
			if !caller.IsSuperAdmin() {
				return nil, huma.Error403Forbidden("only super-admin may grant admin")
			}
		default:
			return nil, huma.Error422UnprocessableEntity("invalid role: " + r)
		}
		out = append(out, rbac.Role(r))
	}
	return out, nil
}

