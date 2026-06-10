// Package user owns the users / roles / auth_identities tables. It is the single place
// that reads and writes accounts, used both by the auth flows and by org-admin user
// management. It imports rbac but no other domain package.
package user

import (
	"time"

	"github.com/google/uuid"

	"github.com/your-org/go-react-starter/backend/internal/rbac"
)

type User struct {
	ID        uuid.UUID
	OrgID     *uuid.UUID
	Name      string
	Email     *string
	Phone     *string
	Status    string
	Roles     []rbac.Role
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUser is the input for creating an account.
type NewUser struct {
	OrgID        *uuid.UUID
	Name         string
	Email        *string
	Phone        *string
	PasswordHash *string
	Roles        []rbac.Role
}

// Identity links a login provider to a user (provider, subject) -> user_id.
type Identity struct {
	Provider string // password | google | phone
	Subject  string // email | google sub | E.164 phone
}

