-- Tamper-evident audit trail. A trigger blocks UPDATE/DELETE so rows are append-only.
CREATE TABLE audit_logs (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid,
    actor_id      uuid NOT NULL,
    action        text NOT NULL,
    resource_type text NOT NULL,
    resource_id   text NOT NULL,
    purpose       text,
    ip            text,
    user_agent    text,
    metadata      jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_resource_idx ON audit_logs (resource_type, resource_id);
CREATE INDEX audit_logs_org_idx ON audit_logs (org_id, created_at);

CREATE FUNCTION audit_logs_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_logs_no_mutate
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_immutable();

---- create above / drop below ----

DROP TRIGGER audit_logs_no_mutate ON audit_logs;
DROP FUNCTION audit_logs_immutable;
DROP TABLE audit_logs;
