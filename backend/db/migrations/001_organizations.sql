-- Organizations are the tenant boundary. Soft-deleted via deleted_at (never hard-deleted).
CREATE TABLE organizations (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text NOT NULL,
    type          text NOT NULL DEFAULT 'clinic',
    address       jsonb NOT NULL DEFAULT '{}'::jsonb,
    contact_email text,
    contact_phone text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

---- create above / drop below ----

DROP TABLE organizations;
