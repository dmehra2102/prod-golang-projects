package domain

import (
	"time"

	"github.com/google/uuid"
)

// ------------- USER --------------------

type UserID = uuid.UUID

type User struct {
	ID             UserID      `json:"id" db:"id"`
	Email          string      `json:"email" db:"email"`
	Username       string      `json:"username" db:"username"`
	DisplayName    string      `json:"display_name" db:"display_name"`
	Bio            string      `json:"bio" db:"bio"`
	AvatarURL      string      `json:"avatar_url" db:"avatar_url"`
	PasswordHash   string      `json:"-" db:"password_hash"`
	TravelStyle    TravelStyle `json:"travel_style" db:"travel_style"`
	Languages      []string    `json:"languages" db:"languages"`
	CountryCode    string      `json:"country_code" db:"country_code"`
	Verified       bool        `json:"verified" db:"verified"`
	FollowerCount  int         `json:"follower_count" db:"follower_count"`
	FollowingCount int         `json:"following_count" db:"following_count"`
	TripCount      int         `json:"trip_count" db:"trip_count"`
	LastLocation   *GeoPoint   `json:"last_location,omitempty" db:"last_location"`
	CreatedAt      time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at" db:"updated_at"`
	DeletedAt      *time.Time  `json:"-" db:"deleted_at"`
}

type TravelStyle string

const (
	TravelStyleBackpacker   TravelStyle = "backpacker"
	TravelStyleLuxury       TravelStyle = "luxury"
	TravelStyleAdventure    TravelStyle = "adventure"
	TravelStyleCultural     TravelStyle = "cultural"
	TravelStyleFoodie       TravelStyle = "foodie"
	TravelStyleDigitalNomad TravelStyle = "digital_nomad"
	TravelStyleSolo         TravelStyle = "solo"
	TravelStyleFamily       TravelStyle = "family"
)

type GeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type UserFollow struct {
	FollowerID UserID    `json:"follower_id" db:"follower_id"`
	FolloweeID UserID    `json:"followee_id" db:"followee_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// ---------------- TRIP ----------------------

type TripID = uuid.UUID

type Trip struct {
	ID            TripID        `json:"id" db:"id"`
	UserID        UserID        `json:"user_id" db:"user_id"`
	Title         string        `json:"title" db:"title"`
	Description   string        `json:"description" db:"description"`
	CoverImageURL string        `json:"cover_image_url" db:"cover_image_url"`
	Status        TripStatus    `json:"status" db:"status"`
	Visibility    Visibility    `json:"visibility" db:"visibility"`
	StartDate     time.Time     `json:"start_date" db:"start_date"`
	EndDate       *time.Time    `json:"end_date,omitempty" db:"end_date"`
	Budget        *Money        `json:"budget,omitempty" db:"budget"`
	Tags          []string      `json:"tags" db:"tags"`
	Destinations  []Destination `json:"destinations" db:"-"`
	LikeCount     int           `json:"like_count" db:"like_count"`
	CommentCount  int           `json:"comment_count" db:"comment_count"`
	ShareCount    int           `json:"share_count" db:"share_count"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at" db:"updated_at"`
	DeletedAt     *time.Time    `json:"-" db:"deleted_at"`
}

type TripStatus string

const (
	TripStatusPlanning  TripStatus = "planning"
	TripStatusActive    TripStatus = "active"
	TripStatusCompleted TripStatus = "completed"
	TripStatusCancelled TripStatus = "cancelled"
)

type Visibility string

const (
	VisibilityPublic    Visibility = "public"
	VisibilityFollowers Visibility = "followers"
	VisibilityPrivate   Visibility = "private"
)

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"` // ISO 4217
}

type Destination struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TripID      TripID     `json:"trip_id" db:"trip_id"`
	Name        string     `json:"name" db:"name"`
	Country     string     `json:"country" db:"country"`
	Location    GeoPoint   `json:"location" db:"location"`
	ArrivalDate time.Time  `json:"arrival_date" db:"arrival_date"`
	DepartDate  *time.Time `json:"depart_date,omitempty" db:"depart_date"`
	Notes       string     `json:"notes" db:"notes"`
	OrderIndex  int        `json:"order_index" db:"order_index"`
}

// ---------------- FEED --------------------
type FeedItemID = uuid.UUID

type FeedItem struct {
	ID         FeedItemID   `json:"id" db:"id"`
	UserID     UserID       `json:"user_id" db:"user_id"`
	ActorID    UserID       `json:"actor_id" db:"actor_id"`
	ActionType FeedAction   `json:"action_type" db:"action_type"`
	TargetType string       `json:"target_type" db:"target_type"`
	TargetID   uuid.UUID    `json:"target_id" db:"target_id"`
	Metadata   FeedMetadata `json:"metadata" db:"metadata"`
	Read       bool         `json:"read" db:"read"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
}

type FeedAction string

const (
	FeedActionTripCreated    FeedAction = "trip_created"
	FeedActionTripUpdated    FeedAction = "trip_updated"
	FeedActionTripLiked      FeedAction = "trip_liked"
	FeedActionTripCommented  FeedAction = "trip_commented"
	FeedActionTripShared     FeedAction = "trip_shared"
	FeedActionUserFollowed   FeedAction = "user_followed"
	FeedActionNearbyTraveler FeedAction = "nearby_traveler"
	FeedActionBuddyMatch     FeedAction = "buddy_match"
)

type FeedMetadata map[string]any

// ---------------- Notification ------------------

type NotificationID = uuid.UUID

type Notification struct {
	ID        NotificationID      `json:"id" db:"id"`
	UserID    UserID              `json:"user_id" db:"user_id"`
	Type      NotificationType    `json:"type" db:"type"`
	Title     string              `json:"title" db:"title"`
	Body      string              `json:"body" db:"body"`
	Data      map[string]string   `json:"data" db:"data"`
	Channel   NotificationChannel `json:"channel" db:"channel"`
	Read      bool                `json:"read" db:"read"`
	SentAt    *time.Time          `json:"sent_at,omitempty" db:"sent_at"`
	CreatedAt time.Time           `json:"created_at" db:"created_at"`
}

type NotificationType string

const (
	NotifTypeTripLike     NotificationType = "trip_like"
	NotifTypeTripComment  NotificationType = "trip_comment"
	NotifTypeNewFollower  NotificationType = "new_follower"
	NotifTypeBuddyMatch   NotificationType = "buddy_match"
	NotifTypeNearbyAlert  NotificationType = "nearby_alert"
	NotifTypeTripReminder NotificationType = "trip_reminder"
)

type NotificationChannel string

const (
	ChannelPush  NotificationChannel = "push"
	ChannelEmail NotificationChannel = "email"
	ChannelInApp NotificationChannel = "in_app"
)

// ---------- Matching ----------

type MatchID = uuid.UUID

type BuddyMatch struct {
	ID          MatchID     `json:"id" db:"id"`
	UserAID     UserID      `json:"user_a_id" db:"user_a_id"`
	UserBID     UserID      `json:"user_b_id" db:"user_b_id"`
	Score       float64     `json:"score" db:"score"`
	MatchReason []string    `json:"match_reason" db:"match_reason"`
	Status      MatchStatus `json:"status" db:"status"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

type MatchStatus string

const (
	MatchStatusPending  MatchStatus = "pending"
	MatchStatusAccepted MatchStatus = "accepted"
	MatchStatusDeclined MatchStatus = "declined"
)

// ---------- Comment ----------

type CommentID = uuid.UUID

type Comment struct {
	ID        CommentID  `json:"id" db:"id"`
	TripID    TripID     `json:"trip_id" db:"trip_id"`
	UserID    UserID     `json:"user_id" db:"user_id"`
	ParentID  *CommentID `json:"parent_id,omitempty" db:"parent_id"`
	Body      string     `json:"body" db:"body"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"-" db:"deleted_at"`

	// Denormalized author info — populated by joins, not stored in comments table.
	AuthorName     string `json:"author_name,omitempty" db:"-"`
	AuthorUsername string `json:"author_username,omitempty" db:"-"`
	AuthorAvatar   string `json:"author_avatar,omitempty" db:"-"`
}

// ---------- Pagination ----------

type Cursor struct {
	After  string `json:"after,omitempty"`
	Before string `json:"before,omitempty"`
	Limit  int    `json:"limit"`
}

func (c *Cursor) Normalize() {
	if c.Limit <= 0 || c.Limit > 100 {
		c.Limit = 20
	}
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	TotalCount int64  `json:"total_count,omitempty"`
}
