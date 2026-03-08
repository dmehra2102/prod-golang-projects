package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	Source        string          `json:"source"`
	Subject       string          `json:"subject"`
	CorrelationID string          `json:"correlation_id"`
	CausationID   string          `json:"causation_id,omitempty"`
	Timestamp     time.Time       `json:"timestamp"`
	Version       int             `json:"version"`
	Data          json.RawMessage `json:"data"`
	Metadata      EventMetadata   `json:"metadata"`
}

type EventMetadata struct {
	UserID     string `json:"user_id,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	SpanID     string `json:"span_id,omitempty"`
	RetryCount int    `json:"retry_count"`
}

type EventType string

const (
	// User events
	EventUserRegistered      EventType = "user.registered"
	EventUserProfileUpdated  EventType = "user.profile_updated"
	EventUserLocationUpdated EventType = "user.location_updated"
	EventUserFollowed        EventType = "user.followed"
	EventUserUnfollowed      EventType = "user.unfollowed"
	EventUserDeactivated     EventType = "user.deactivated"

	// Trip events
	EventTripCreated   EventType = "trip.created"
	EventTripUpdated   EventType = "trip.updated"
	EventTripPublished EventType = "trip.published"
	EventTripCompleted EventType = "trip.completed"
	EventTripCancelled EventType = "trip.cancelled"
	EventTripLiked     EventType = "trip.liked"
	EventTripUnliked   EventType = "trip.unliked"
	EventTripCommented EventType = "trip.commented"
	EventTripShared    EventType = "trip.shared"

	// Feed events
	EventFeedItemCreated EventType = "feed.item_created"

	// Notification events
	EventNotificationCreated   EventType = "notification.created"
	EventNotificationDelivered EventType = "notification.delivered"
	EventNotificationFailed    EventType = "notification.failed"

	// Matching events
	EventMatchComputed EventType = "match.computed"
	EventMatchAccepted EventType = "match.accepted"
	EventMatchDeclined EventType = "match.declined"

	// Analytics events
	EventPageViewed     EventType = "analytics.page_viewed"
	EventTripViewed     EventType = "analytics.trip_viewed"
	EventSearchExecuted EventType = "analytics.search_executed"
)

func NewEvent(eventType EventType, source, subject string, data any, meta EventMetadata) (*Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	correlationID := meta.TraceID
	if correlationID == "" {
		correlationID = uuid.NewString()
	}

	return &Event{
		ID:            uuid.NewString(),
		Type:          eventType,
		Source:        source,
		Subject:       subject,
		CorrelationID: correlationID,
		Timestamp:     time.Now().UTC(),
		Version:       1,
		Data:          raw,
		Metadata:      meta,
	}, nil
}

func (e *Event) Key() string {
	return e.Subject
}

// ---------- Event Payloads ----------
type UserRegisteredPayload struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	TravelStyle string `json:"travel_style"`
}

type UserLocationUpdatedPayload struct {
	UserID    string  `json:"user_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city,omitempty"`
	Country   string  `json:"country,omitempty"`
}

type UserFollowedPayload struct {
	FollowerID   string `json:"follower_id"`
	FolloweeID   string `json:"followee_id"`
	FollowerName string `json:"follower_name"`
}

type TripCreatedPayload struct {
	TripID       string   `json:"trip_id"`
	UserID       string   `json:"user_id"`
	Title        string   `json:"title"`
	Destinations []string `json:"destinations"`
	StartDate    string   `json:"start_date"`
	Visibility   string   `json:"visibility"`
	Tags         []string `json:"tags"`
}

type TripLikedPayload struct {
	TripID    string `json:"trip_id"`
	LikerID   string `json:"liker_id"`
	OwnerID   string `json:"owner_id"`
	TripTitle string `json:"trip_title"`
}

type TripCommentedPayload struct {
	TripID      string `json:"trip_id"`
	CommentID   string `json:"comment_id"`
	CommenterID string `json:"commenter_id"`
	OwnerID     string `json:"owner_id"`
	TripTitle   string `json:"trip_title"`
	CommentBody string `json:"comment_body"`
}

type MatchComputedPayload struct {
	MatchID     string   `json:"match_id"`
	UserAID     string   `json:"user_a_id"`
	UserBID     string   `json:"user_b_id"`
	Score       float64  `json:"score"`
	MatchReason []string `json:"match_reason"`
}

type NotificationPayload struct {
	NotificationID string            `json:"notification_id"`
	UserID         string            `json:"user_id"`
	Type           string            `json:"type"`
	Title          string            `json:"title"`
	Body           string            `json:"body"`
	Channel        string            `json:"channel"`
	Data           map[string]string `json:"data"`
}

// EventHandler is implemented by consumers that process domain events.
type EventHandler interface {
	Handle(event *Event) error
	EventTypes() []EventType
}
