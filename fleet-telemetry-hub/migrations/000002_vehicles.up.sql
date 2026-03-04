CREATE TABLE IF NOT EXISTS vehicles (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID    NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    vin         TEXT    NOT NULL,
    plate       TEXT,
    make        TEXT,
    model       TEXT,
    year        INTEGER CHECK(year BETWEEN 1900 AND 2100),
    type        TEXT    NOT NULL DEFAULT 'truck'
                        CHECK (type IN ('truck', 'van', 'car', 'motorcycle', 'heavy_equipment')),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    metadata    JSONB   NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)

CREATE UNIQUE INDEX idx_vehicles_tenant_vin ON vehicles (tenant_id, vin);
CREATE INDEX        idx_vehicles_tenant_id  ON vehicles (tenant_id);
CREATE INDEX        idx_vehicles_is_active  ON vehicles (tenant_id, is_active) WHERE is_active = TRUE;
CREATE INDEX        idx_vehicles_last_seen  ON vehicles (last_seen_at DESC NULLS LAST);

CREATE TRIGGER vehicles_updated_at
    BEFORE UPDATE ON vehicles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE vehicles IS 'Vehicle registry scoped per tenant. VIN must be unique within a tenant.';
COMMENT ON COLUMN vehicles.vin IS 'Vehicle Identification Number (17-char industry standard).';