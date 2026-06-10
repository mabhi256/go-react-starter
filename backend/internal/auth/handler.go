package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/your-org/go-react-starter/backend/internal/apiutil"
)

// Handler exposes the auth endpoints over Huma.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

var bearer = []map[string][]string{{"bearer": {}}}

func (h *Handler) Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-login", Method: http.MethodPost, Path: "/auth/login",
		Summary: "Login with email and password", Tags: []string{"Auth"},
	}, h.login)

	huma.Register(api, huma.Operation{
		OperationID: "auth-google", Method: http.MethodPost, Path: "/auth/google",
		Summary: "Login with a Google ID token", Tags: []string{"Auth"},
	}, h.google)

	huma.Register(api, huma.Operation{
		OperationID: "auth-otp-request", Method: http.MethodPost, Path: "/auth/otp/request",
		Summary: "Request a phone OTP", Tags: []string{"Auth"},
	}, h.otpRequest)

	huma.Register(api, huma.Operation{
		OperationID: "auth-otp-verify", Method: http.MethodPost, Path: "/auth/otp/verify",
		Summary: "Verify a phone OTP", Tags: []string{"Auth"},
	}, h.otpVerify)

	huma.Register(api, huma.Operation{
		OperationID: "auth-refresh", Method: http.MethodPost, Path: "/auth/refresh",
		Summary: "Exchange a refresh token for a new token pair", Tags: []string{"Auth"},
	}, h.refresh)

	huma.Register(api, huma.Operation{
		OperationID: "auth-logout", Method: http.MethodPost, Path: "/auth/logout",
		Summary: "Revoke a refresh token", Tags: []string{"Auth"}, Security: bearer,
	}, h.logout)

	huma.Register(api, huma.Operation{
		OperationID: "auth-me", Method: http.MethodGet, Path: "/auth/me",
		Summary: "Current authenticated identity", Tags: []string{"Auth"}, Security: bearer,
	}, h.me)

	huma.Register(api, huma.Operation{
		OperationID: "auth-change-password", Method: http.MethodPut, Path: "/auth/password",
		Summary: "Change password for the authenticated user", Tags: []string{"Auth"}, Security: bearer,
	}, h.changePassword)

	huma.Register(api, huma.Operation{
		OperationID: "auth-forgot-password", Method: http.MethodPost, Path: "/auth/password/forgot",
		Summary: "Send a password reset email", Tags: []string{"Auth"},
	}, h.forgotPassword)

	huma.Register(api, huma.Operation{
		OperationID: "auth-password-reset", Method: http.MethodPost, Path: "/auth/password/reset",
		Summary: "Reset password using a token from email", Tags: []string{"Auth"},
	}, h.passwordReset)
}

// ---- I/O types ----

type tokenOutput struct {
	Body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type" example:"Bearer"`
		ExpiresIn    int    `json:"expires_in" doc:"Access token lifetime in seconds"`
	}
}

func toTokenOutput(p TokenPair) *tokenOutput {
	out := &tokenOutput{}
	out.Body.AccessToken = p.AccessToken
	out.Body.RefreshToken = p.RefreshToken
	out.Body.TokenType = "Bearer"
	out.Body.ExpiresIn = p.ExpiresIn
	return out
}

type loginInput struct {
	Body struct {
		Email    string `json:"email" format:"email"`
		Password string `json:"password" minLength:"1"`
	}
}

type googleInput struct {
	Body struct {
		IDToken string `json:"id_token" minLength:"1" doc:"Google ID token from the client"`
	}
}

type otpRequestInput struct {
	Body struct {
		Phone string `json:"phone" minLength:"4" doc:"Registered phone number"`
	}
}

type otpVerifyInput struct {
	Body struct {
		Phone string `json:"phone" minLength:"4"`
		Code  string `json:"code" minLength:"4" maxLength:"8"`
	}
}

type refreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token" minLength:"1"`
	}
}

type messageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

type changePasswordInput struct {
	Body struct {
		CurrentPassword string `json:"current_password" minLength:"1"`
		NewPassword     string `json:"new_password" minLength:"8"`
	}
}

type passwordResetRequestInput struct {
	Body struct {
		Email string `json:"email" format:"email"`
	}
}

type passwordResetInput struct {
	Body struct {
		Token       string `json:"token" minLength:"1"`
		NewPassword string `json:"new_password" minLength:"8"`
	}
}

type meOutput struct {
	Body struct {
		UserID string   `json:"user_id"`
		OrgID  *string  `json:"org_id"`
		Roles  []string `json:"roles"`
	}
}

// ---- handlers ----

func (h *Handler) login(ctx context.Context, in *loginInput) (*tokenOutput, error) {
	pair, err := h.svc.LoginPassword(ctx, in.Body.Email, in.Body.Password)
	return h.tokenResult(pair, err)
}

func (h *Handler) google(ctx context.Context, in *googleInput) (*tokenOutput, error) {
	pair, err := h.svc.LoginGoogle(ctx, in.Body.IDToken)
	return h.tokenResult(pair, err)
}

func (h *Handler) otpRequest(ctx context.Context, in *otpRequestInput) (*messageOutput, error) {
	if err := h.svc.RequestOTP(ctx, in.Body.Phone); err != nil {
		return nil, huma.Error500InternalServerError("could not send OTP")
	}
	out := &messageOutput{}
	out.Body.Message = "If the number is registered, an OTP has been sent."
	return out, nil
}

func (h *Handler) otpVerify(ctx context.Context, in *otpVerifyInput) (*tokenOutput, error) {
	pair, err := h.svc.VerifyOTP(ctx, in.Body.Phone, in.Body.Code)
	return h.tokenResult(pair, err)
}

func (h *Handler) refresh(ctx context.Context, in *refreshInput) (*tokenOutput, error) {
	pair, err := h.svc.Refresh(ctx, in.Body.RefreshToken)
	return h.tokenResult(pair, err)
}

func (h *Handler) logout(ctx context.Context, in *refreshInput) (*messageOutput, error) {
	if err := h.svc.Logout(ctx, in.Body.RefreshToken); err != nil {
		return nil, huma.Error500InternalServerError("logout failed")
	}
	out := &messageOutput{}
	out.Body.Message = "logged out"
	return out, nil
}

func (h *Handler) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	id := apiutil.Identity(ctx)
	out := &meOutput{}
	out.Body.UserID = id.UserID.String()
	if id.OrgID != nil {
		s := id.OrgID.String()
		out.Body.OrgID = &s
	}
	for _, r := range id.Roles {
		out.Body.Roles = append(out.Body.Roles, string(r))
	}
	return out, nil
}

func (h *Handler) changePassword(ctx context.Context, in *changePasswordInput) (*messageOutput, error) {
	id := apiutil.Identity(ctx)
	if err := h.svc.ChangePassword(ctx, id.UserID, in.Body.CurrentPassword, in.Body.NewPassword); err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("current password is incorrect")
		}
		return nil, huma.Error500InternalServerError("could not change password")
	}
	out := &messageOutput{}
	out.Body.Message = "password updated"
	return out, nil
}

func (h *Handler) forgotPassword(ctx context.Context, in *passwordResetRequestInput) (*messageOutput, error) {
	if err := h.svc.ForgotPassword(ctx, in.Body.Email); err != nil {
		return nil, huma.Error500InternalServerError("could not send reset email")
	}
	out := &messageOutput{}
	out.Body.Message = "If that email is registered, a reset link has been sent."
	return out, nil
}

func (h *Handler) passwordReset(ctx context.Context, in *passwordResetInput) (*messageOutput, error) {
	if err := h.svc.ResetPassword(ctx, in.Body.Token, in.Body.NewPassword); err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return nil, huma.Error400BadRequest("invalid or expired reset token")
		}
		return nil, huma.Error500InternalServerError("could not reset password")
	}
	out := &messageOutput{}
	out.Body.Message = "password reset successfully"
	return out, nil
}

func (h *Handler) tokenResult(pair TokenPair, err error) (*tokenOutput, error) {
	if errors.Is(err, ErrUnauthorized) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("login failed")
	}
	return toTokenOutput(pair), nil
}

