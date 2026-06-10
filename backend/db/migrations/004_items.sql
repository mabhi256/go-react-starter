CREATE TABLE items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations(id),
    name        text NOT NULL,
    description text,
    created_by  uuid REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);
CREATE INDEX items_org_idx ON items (org_id) WHERE deleted_at IS NULL;

---- create above / drop below ----

DROP TABLE items;
