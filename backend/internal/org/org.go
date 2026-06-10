// Package org manages organizations (tenants). Only super-admins may manage them. Deletes
// are soft (deleted_at) so historical clinical data is never orphaned.
package org

import (
	"time"

	"github.com/google/uuid"
)

// Address is the org's postal address, stored as jsonb.
type Address struct {
	Line     string `json:"line,omitempty"`
	City     string `json:"city,omitempty"`
	District string `json:"district,omitempty"`
	State    string `json:"state,omitempty"`
	Pincode  string `json:"pincode,omitempty"`
}

type Organization struct {
	ID           uuid.UUID
	Name         string
	Type         string
	Status       string
	Address      Address
	ContactEmail *string
	ContactPhone *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type NewOrg struct {
	Name         string
	Type         string
	Address      Address
	ContactEmail *string
	ContactPhone *string
}

// Patch carries optional updates; nil fields are left unchanged.
type Patch struct {
	Name         *string
	Type         *string
	Status       *string
	Address      *Address
	ContactEmail *string
	ContactPhone *string
}
