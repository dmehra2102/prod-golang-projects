ALTER TABLE telemetry DROP CONSTRAINT IF EXISTS fk_telemetry_vehicle;
ALTER TABLE telemetry DROP CONSTRAINT IF EXISTS fk_telemetry_tenant;
DROP TABLE IF EXISTS telemetry CASCADE;