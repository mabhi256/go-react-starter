-- Users belong to one org (org_id NULL = platform super-admin). A user may authenticate
-- via several providers (auth_identities) and hold several roles (user_roles).
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid REFERENCES organizations(id),
    name          text NOT NULL,
    email         text,
    phone         text,
    password_hash text,
    status        text NOT NULL DEFAULT 'active',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
-- Email / phone are globally unique among live accounts (login identifiers).
CREATE UNIQUE INDEX users_email_uq ON users (lower(email)) WHERE email IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX users_phone_uq ON users (phone)        WHERE phone IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX users_org_idx ON users (org_id) WHERE deleted_at IS NULL;

CREATE TABLE roles (
    key         text PRIMARY KEY,
    description text NOT NULL
);
INSERT INTO roles (key, description) VALUES
    ('super_admin', 'Platform administrator across all organizations'),
    ('admin',       'Administrator of an organization'),
    ('user',        'Standard user within an organization');

CREATE TABLE user_roles (
    user_id  uuid NOT NULL REFERENCES users(id),
    role_key text NOT NULL REFERENCES roles(key),
    PRIMARY KEY (user_id, role_key)
);

CREATE TABLE auth_identities (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users(id),
    provider         text NOT NULL,  -- 'password' | 'google' | 'phone'
    provider_subject text NOT NULL,  -- email | google sub | E.164 phone
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_subject)
);
CREATE INDEX auth_identities_user_idx ON auth_identities (user_id);

---- create above / drop below ----

DROP TABLE auth_identities;
DROP TABLE user_roles;
DROP TABLE roles;
DROP TABLE users;
