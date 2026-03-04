CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS tenants (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL,
    slug            TEXT        NOT NULL,
    api_key_hash    TEXT        NOT NULL,
    plan            TEXT        NOT NULL DEFAULT 'standard' 
                                CHECK (plan IN ('standard', 'professional', 'enterprise')),
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    max_vehicles    INTEGER     NOT NULL DEFAULT 100,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
)

CREATE UNIQUE INDEX idx_tenants_slug ON tenants (slug);
CREATE UNIQUE INDEX idx_tenants_api_key_hash ON tenants (api_key_hash);
CREATE INDEX        idx_tenants_is_active ON tenants (is_active) WHERE is_active = TRUE;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE tenants IS 'Multi-tenant isolation root - one row per fleet operator.';