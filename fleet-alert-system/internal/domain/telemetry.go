package domain

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventTypeGPSUpdate      EventType = "GPS_UPDATE"
	EventTypeIgnitionOn     EventType = "IGNITION_ON"
	EventTypeIgnitionOff    EventType = "IGNITION_OFF"
	EventTypeHarshBake      EventType = "HARSH_BRAKE"
	EventTypeHarshAccel     EventType = "HARSH_ACCEL"
	EventTypeHarshCornering EventType = "HARSH_CORNERING"
	EventTypeDTC            EventType = "DTC"
	EventTypeFuelLevel      EventType = "FUEL_LEVEL"
	EventTypePanicButton    EventType = "PANIC_BUTTON"
	EventTypeIdleStart      EventType = "IDLE_START"
	EventTypeIdleEnd        EventType = "IDLE_END"
	EventTypeTripStart      EventType = "TRIP_START"
	EventTypeTripEnd        EventType = "TRIP_END"
	EventTypeTemperature    EventType = "TEMPERATURE"
	EventTypeRPM            EventType = "RPM"
)

// TelemetryEvent is the canonical representation of any event
// from a telematics device. All device-specific formats are
// normalized to this struct by the ingestor before being queued.
type TelemetryEvent struct {
	EventID  string `json:"event_id"`
	DeviceID string `json:"device_id"`
	AssetID  string `json:"asset_id"`
	FleetID  string `json:"fleet_id"`
	OrgID    string `json:"org_id"`

	ReceivedAt time.Time `json:"received_at"`
	OccurredAt time.Time `json:"occurred_at"`

	EventType EventType `json:"event_type"`

	Payload json.RawMessage `json:"payload"`

	SchemaVersion string `json:"schema_version"`
	SourceModel   string `json:"source_model"`
	FirmwareVer   string `json:"firmware_ver"`

	IngestorID string `json:"ingestor_id"`
}

type GPSPayload struct {
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	AltMeters  float64 `json:"alt_m"`
	SpeedKPH   float64 `json:"speed_kph"`
	Heading    float64 `json:"heading"`    // degrees 0–360
	AccuracyM  float64 `json:"accuracy_m"` // GPS accuracy radius
	Satellites int     `json:"satellites"`
	HDOP       float64 `json:"hdop"` // horizontal dilution of precision
}

// HarshEventPayload is carried by EventTypeHarshBrake,
// EventTypeHarshAccel, and EventTypeHarshCornering.
type HarshEventPayload struct {
	Type       string  `json:"type"`
	GForce     float64 `json:"g_force"`
	DurationMs int     `json:"duration_ms"`
	SpeedKPH   float64 `json:"speed_kph"`
}

// DTCPayload is carried by EventTypeDTC (OBD-II fault codes).
type DTCPayload struct {
	Code        string `json:"code"` // e.g. "P0301"
	Description string `json:"description"`
	Severity    string `json:"severity"` // pending | confirmed | permanent
	Protocol    string `json:"protocol"` // OBD-II | J1939 | etc.
}

// FuelPayload is carried by EventTypeFuelLevel.
type FuelPayload struct {
	LevelPct           float64 `json:"level_pct"`
	LevelLiters        float64 `json:"level_liters"`
	ConsumptionRateLPH float64 `json:"consumption_lph"`
}

// RPMPayload is carried by EventTypeRPM.
type RPMPayload struct {
	RPM      float64 `json:"rpm"`
	Throttle float64 `json:"throttle_pct"`
}

// TemperaturePayload is carried by EventTypeTemperature.
// Used for reefer units, engine coolant, etc.
type TemperaturePayload struct {
	SensorID   string  `json:"sensor_id"`
	SensorName string  `json:"sensor_name"`
	TempC      float64 `json:"temp_c"`
}

type EnrichedEvent struct {
	TelemetryEvent

	// Asset Context added by enricher
	Asset *AssetContext `json:"asset,omitempty"`

	// Driver currently assigned to this asset
	Driver *DriverContext `json:"driver,omitempty"`

	// Human-readable location derived from lat/lon
	Address *AddressContext `json:"address,omitempty"`

	CurrentGeofences  []GeofenceRef `json:"current_geofences"`
	PreviousGeofences []GeofenceRef `json:"previous_geofences"`

	// Trip context
	TripID         string        `json:"trip_id,omitempty"`
	TripDuration   time.Duration `json:"trip_duration,omitempty"`
	TripDistanceKM float64       `json:"trip_distance_km,omitempty"`

	// Derived metrics computed during enrichment
	IdleDurationSecs int `json:"idle_duration_secs,omitempty"`
}

type AssetContext struct {
	AssetID      string   `json:"asset_id"`
	Name         string   `json:"name"`
	LicensePlate string   `json:"license_plate"`
	VIN          string   `json:"vin"`
	Make         string   `json:"make"`
	Model        string   `json:"model"`
	Year         int      `json:"year"`
	Tags         []string `json:"tags"`
}

type DriverContext struct {
	DriverID string `json:"driver_id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

type AddressContext struct {
	FormattedAddress string  `json:"formatted_address"`
	City             string  `json:"city"`
	State            string  `json:"state"`
	Country          string  `json:"country"`
	PostalCode       string  `json:"postal_code"`
	Lat              float64 `json:"lat"`
	Lon              float64 `json:"lon"`
}

// GeofenceRef is a lightweight reference to a geofence membership.
type GeofenceRef struct {
	GeofenceID string `json:"geofence_id"`
	Name       string `json:"name"`
}
