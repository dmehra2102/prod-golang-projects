-- -------- Telemetry (range-partitioned by recorded_at) ------------------
CREATE TABLE telemetry (
    id              UUID        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL,
    vehicle_id      UUID        NOT NULL,

    recorded_at     TIMESTAMPTZ NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    sequence_number BIGINT      NOT NULL CHECK (sequence_number >= 0),

    -- GPS
    latitude        DOUBLE PRECISION CHECK (latitude BETWEEN -90 AND 90),
    longitude       DOUBLE PRECISION CHECK (longitude BETWEEN -180 AND 180),
    altitude_m      REAL,
    heading_deg     REAL CHECK (heading_deg >= 0 AND heading_deg < 360),
    accuracy_m      REAL CHECK (accuracy_m >= 0),

    -- Motion
    speed_kmh       REAL CHECK (speed_kmh >= 0),
    odometer_km     REAL CHECK (odometer_km >= 0),

    -- Engine
    rpm             INTEGER CHECK (rpm >= 0),
    engine_temp_c   REAL,
    fuel_level_pct  REAL CHECK (fuel_level_pct BETWEEN 0 AND 100),
    fuel_consumed_l REAL CHECK (fuel_consumed_l >= 0),

    -- Diagnostics
    battery_v       REAL,
    ignition_on     BOOLEAN,

    -- Future-proofing
    extra           JSONB       NOT NULL DEFAULT '{}'::jsonb,

    -- Surrogate PK
    PRIMARY KEY (id, recorded_at),

    -- Hard deduplication rule
    UNIQUE (vehicle_id, recorded_at, sequence_number),

    -- Logical sanity check
    CHECK (received_at >= recorded_at)

) PARTITION BY RANGE (recorded_at);

CREATE TABLE telemetry_2026_01 PARTITION OF telemetry
FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE telemetry_2026_02 PARTITION OF telemetry
FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');