package org

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
	"github.com/your-org/go-react-starter/backend/internal/user"
)

// Handler exposes super-admin organization management. Every operation requires the
// super_admin role; org CRUD logic is thin so it lives here rather than a pass-through service.
type Handler struct {
	repo      *Repo
	userRepo  *user.Repo
	audit     audit.Recorder
}

func NewHandler(repo *Repo, userRepo *user.Repo, rec audit.Recorder) *Handler {
	return &Handler{repo: repo, userRepo: userRepo, audit: rec}
}

var bearer = []map[string][]string{{"bearer": {}}}

func (h *Handler) Register(api huma.API) {
	tag := []string{"Organizations (super-admin)"}
	huma.Register(api, huma.Operation{OperationID: "org-create", Method: http.MethodPost, Path: "/admin/orgs",
		Summary: "Create an organization with an initial org-admin", Tags: tag, Security: bearer, DefaultStatus: http.StatusCreated}, h.create)
	huma.Register(api, huma.Operation{OperationID: "org-list", Method: http.MethodGet, Path: "/admin/orgs",
		Summary: "List organizations", Tags: tag, Security: bearer}, h.list)
	huma.Register(api, huma.Operation{OperationID: "org-get", Method: http.MethodGet, Path: "/admin/orgs/{id}",
		Summary: "Get an organization", Tags: tag, Security: bearer}, h.get)
	huma.Register(api, huma.Operation{OperationID: "org-update", Method: http.MethodPatch, Path: "/admin/orgs/{id}",
		Summary: "Update an organization", Tags: tag, Security: bearer}, h.update)
	huma.Register(api, huma.Operation{OperationID: "org-delete", Method: http.MethodDelete, Path: "/admin/orgs/{id}",
		Summary: "Soft-delete an organization", Tags: tag, Security: bearer, DefaultStatus: http.StatusNoContent}, h.delete)

	adminTag := []string{"Admin Users (super-admin)"}
	huma.Register(api, huma.Operation{OperationID: "admin-create-user", Method: http.MethodPost, Path: "/admin/users",
		Summary: "Create a super-admin or org-admin", Tags: adminTag, Security: bearer, DefaultStatus: http.StatusCreated}, h.createAdmin)
}

// ---- I/O ----

type orgBody struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	Address      Address `json:"address"`
	ContactEmail *string `json:"contact_email"`
	ContactPhone *string `json:"contact_phone"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func toBody(o *Organization) orgBody {
	return orgBody{
		ID: o.ID.String(), Name: o.Name, Type: o.Type, Status: o.Status, Address: o.Address,
		ContactEmail: o.ContactEmail, ContactPhone: o.ContactPhone,
		CreatedAt: o.CreatedAt.Format(http.TimeFormat), UpdatedAt: o.UpdatedAt.Format(http.TimeFormat),
	}
}

type orgAdminInput struct {
	Name     string  `json:"name" minLength:"1"`
	Email    string  `json:"email" format:"email"`
	Password *string `json:"password,omitempty" minLength:"8" doc:"If omitted, use forgot-password to set a password before first login"`
}

type orgAdminBody struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type orgCreateBody struct {
	Name         string        `json:"name" minLength:"1"`
	Type         string        `json:"type" default:"clinic"`
	Address      Address       `json:"address"`
	ContactEmail *string       `json:"contact_email" format:"email"`
	ContactPhone *string       `json:"contact_phone"`
	Admin        orgAdminInput `json:"admin" doc:"Initial org-admin user; required so someone can log in"`
}
type createInput struct {
	Body orgCreateBody
}
type orgCreateOutput struct {
	Body struct {
		orgBody
		Admin *orgAdminBody `json:"admin,omitempty"`
	}
}

type orgOutput struct{ Body orgBody }
type orgListBody struct {
	Items []orgBody `json:"items"`
}
type listInput struct {
	Q string `query:"q" doc:"Filter organizations by name (case-insensitive substring match)"`
}
type listOutput struct {
	Body orgListBody
}
type idPath struct {
	ID uuid.UUID `path:"id"`
}
type orgUpdateBody struct {
	Name         *string  `json:"name,omitempty"`
	Type         *string  `json:"type,omitempty"`
	Status       *string  `json:"status,omitempty" enum:"active,inactive"`
	Address      *Address `json:"address,omitempty"`
	ContactEmail *string  `json:"contact_email,omitempty"`
	ContactPhone *string  `json:"contact_phone,omitempty"`
}
type updateInput struct {
	ID   uuid.UUID `path:"id"`
	Body orgUpdateBody
}

// ---- handlers ----

func (h *Handler) create(ctx context.Context, in *createInput) (*orgCreateOutput, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	o, err := h.repo.Create(ctx, NewOrg{
		Name: in.Body.Name, Type: orDefault(in.Body.Type, "clinic"), Address: in.Body.Address,
		ContactEmail: in.Body.ContactEmail, ContactPhone: in.Body.ContactPhone,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("could not create organization")
	}
	apiutil.Audit(ctx, h.audit, "org.create", "organization", o.ID.String())

	out := &orgCreateOutput{}
	out.Body.orgBody = toBody(o)

	var hash *string
	if in.Body.Admin.Password != nil {
		_, span := otel.Tracer("org").Start(ctx, "bcrypt.hash")
		hashed, err := bcrypt.GenerateFromPassword([]byte(*in.Body.Admin.Password), bcrypt.DefaultCost)
		span.End()
		if err != nil {
			return nil, huma.Error500InternalServerError("could not hash password")
		}
		s := string(hashed)
		hash = &s
	}
	email := in.Body.Admin.Email
	u, err := h.userRepo.CreateWithIdentity(ctx, user.NewUser{
		OrgID:        &o.ID,
		Name:         in.Body.Admin.Name,
		Email:        &email,
		PasswordHash: hash,
		Roles:        []rbac.Role{rbac.RoleAdmin},
	}, nil)
	if errors.Is(err, user.ErrConflict) {
		return nil, huma.Error409Conflict("admin email already in use")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("org created but could not create admin user")
	}
	apiutil.Audit(ctx, h.audit, "user.create", "user", u.ID.String())
	out.Body.Admin = &orgAdminBody{ID: u.ID.String(), Email: email}

	return out, nil
}

func (h *Handler) list(ctx context.Context, in *listInput) (*listOutput, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	q := in.Q
	orgs, err := h.repo.List(ctx, q)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list organizations")
	}
	out := &listOutput{}
	for i := range orgs {
		out.Body.Items = append(out.Body.Items, toBody(&orgs[i]))
	}
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *idPath) (*orgOutput, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	o, err := h.repo.GetByID(ctx, in.ID)
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("organization not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not load organization")
	}
	apiutil.Audit(ctx, h.audit, "org.read", "organization", o.ID.String())
	return &orgOutput{Body: toBody(o)}, nil
}

func (h *Handler) update(ctx context.Context, in *updateInput) (*orgOutput, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	o, err := h.repo.Update(ctx, in.ID, Patch{
		Name: in.Body.Name, Type: in.Body.Type, Status: in.Body.Status, Address: in.Body.Address,
		ContactEmail: in.Body.ContactEmail, ContactPhone: in.Body.ContactPhone,
	})
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("organization not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not update organization")
	}
	apiutil.Audit(ctx, h.audit, "org.update", "organization", o.ID.String())
	return &orgOutput{Body: toBody(o)}, nil
}

func (h *Handler) delete(ctx context.Context, in *idPath) (*struct{}, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}
	err := h.repo.SoftDelete(ctx, in.ID)
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("organization not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not delete organization")
	}
	apiutil.Audit(ctx, h.audit, "org.delete", "organization", in.ID.String())
	return nil, nil
}

type adminCreateBody struct {
	Name     string     `json:"name" minLength:"1"`
	Email    string     `json:"email" format:"email"`
	Password *string    `json:"password,omitempty" minLength:"8" doc:"If omitted, use forgot-password before first login"`
	OrgID    *uuid.UUID `json:"org_id,omitempty" doc:"Org to assign as org-admin; omit to create a super-admin"`
}
type adminCreateInput struct{ Body adminCreateBody }
type adminUserOutput struct {
	Body struct {
		ID    string  `json:"id"`
		Email string  `json:"email"`
		Role  string  `json:"role"`
		OrgID *string `json:"org_id,omitempty"`
	}
}

func (h *Handler) createAdmin(ctx context.Context, in *adminCreateInput) (*adminUserOutput, error) {
	if err := apiutil.RequireSuperAdmin(ctx); err != nil {
		return nil, err
	}

	role := rbac.RoleSuperAdmin
	var orgID *uuid.UUID
	if in.Body.OrgID != nil {
		if _, err := h.repo.GetByID(ctx, *in.Body.OrgID); errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("organization not found")
		} else if err != nil {
			return nil, huma.Error500InternalServerError("could not verify organization")
		}
		role = rbac.RoleAdmin
		orgID = in.Body.OrgID
	}

	var hash *string
	if in.Body.Password != nil {
		_, span := otel.Tracer("org").Start(ctx, "bcrypt.hash")
		hashed, err := bcrypt.GenerateFromPassword([]byte(*in.Body.Password), bcrypt.DefaultCost)
		span.End()
		if err != nil {
			return nil, huma.Error500InternalServerError("could not hash password")
		}
		s := string(hashed)
		hash = &s
	}

	email := in.Body.Email
	u, err := h.userRepo.CreateWithIdentity(ctx, user.NewUser{
		OrgID:        orgID,
		Name:         in.Body.Name,
		Email:        &email,
		PasswordHash: hash,
		Roles:        []rbac.Role{role},
	}, nil)
	if errors.Is(err, user.ErrConflict) {
		return nil, huma.Error409Conflict("email already in use")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("could not create admin user")
	}
	apiutil.Audit(ctx, h.audit, "user.create", "user", u.ID.String())

	out := &adminUserOutput{}
	out.Body.ID = u.ID.String()
	out.Body.Email = email
	out.Body.Role = string(role)
	if orgID != nil {
		s := orgID.String()
		out.Body.OrgID = &s
	}
	return out, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

