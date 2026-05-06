package businessnotification

import (
	"encoding/json"
	"fmt"
	"time"
)

type Channel string

const (
	ChannelSystem            Channel = "system"
	ChannelProp              Channel = "prop"
	ChannelGroupNotification Channel = "group_notification"
	ChannelSocialLike        Channel = "social_like"
	ChannelSocialComment     Channel = "social_comment"
	ChannelSocialFollow      Channel = "social_follow"
	ChannelGroupPost         Channel = "group_post"
)

const (
	PropSubTypeUse     = "prop_use"
	PropSubTypeAcquire = "prop_acquire"
	PropSubTypeExpire  = "prop_expire"
)

var defaultSenderByChannel = map[Channel]string{
	ChannelSystem:            "2",
	ChannelProp:              "6",
	ChannelGroupNotification: "7",
	ChannelSocialLike:        "8",
	ChannelSocialComment:     "9",
	ChannelSocialFollow:      "10",
	ChannelGroupPost:         "11",
}

type Payload struct {
	BusinessType string         `json:"businessType"`
	SubType      string         `json:"subType,omitempty"`
	EntityID     string         `json:"entityId,omitempty"`
	ActorUserID  string         `json:"actorUserId,omitempty"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary"`
	Route        string         `json:"route,omitempty"`
	CreatedAt    int64          `json:"createdAt"`
	DedupeKey    string         `json:"dedupeKey,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
}

type Delivery struct {
	CountUnread bool `json:"countUnread"`
	OfflinePush bool `json:"offlinePush"`
}

type Envelope struct {
	Payload  Payload  `json:"payload"`
	Delivery Delivery `json:"delivery"`
}

func NormalizePayload(payload Payload) (Payload, error) {
	if payload.BusinessType == "" {
		return payload, fmt.Errorf("businessType is required")
	}
	if !IsSupportedChannel(Channel(payload.BusinessType)) {
		return payload, fmt.Errorf("unsupported businessType %q", payload.BusinessType)
	}
	if payload.Title == "" {
		return payload, fmt.Errorf("title is required")
	}
	if payload.Summary == "" {
		return payload, fmt.Errorf("summary is required")
	}
	if payload.CreatedAt == 0 {
		payload.CreatedAt = time.Now().UnixMilli()
	}
	if Channel(payload.BusinessType) == ChannelProp && payload.SubType != "" && !IsSupportedPropSubType(payload.SubType) {
		return payload, fmt.Errorf("unsupported prop subType %q", payload.SubType)
	}
	return payload, nil
}

func DefaultSenderID(channel Channel) string {
	return defaultSenderByChannel[channel]
}

func IsSupportedChannel(channel Channel) bool {
	_, ok := defaultSenderByChannel[channel]
	return ok
}

func IsSupportedPropSubType(subType string) bool {
	switch subType {
	case PropSubTypeUse, PropSubTypeAcquire, PropSubTypeExpire:
		return true
	default:
		return false
	}
}

func MarshalEnvelope(payload Payload, delivery Delivery) (string, error) {
	b, err := json.Marshal(Envelope{
		Payload:  payload,
		Delivery: delivery,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
