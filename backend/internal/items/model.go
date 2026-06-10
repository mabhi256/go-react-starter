package items

import (
	"time"

	"github.com/google/uuid"
)

type Item struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Name        string
	Description *string
	CreatedBy   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type NewItem struct {
	OrgID       uuid.UUID
	Name        string
	Description *string
	CreatedBy   *uuid.UUID
}

type Patch struct {
	Name        *string
	Description *string
}
