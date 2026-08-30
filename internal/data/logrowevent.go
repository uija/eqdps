package data

import "time"

type LogRowEventType uint8

const (
	LogRowEventTypeUnknown LogRowEventType = iota
	LogRowEventTypeCast
	LogRowEventTypeDamage
	LogRowEventTypeYourDamageShield
	LogRowEventTypeDamageShield
	LogRowEventTypeYourDamageOverTime
	LogRowEventTypeDamageOverTime
	LogRowEventTypeExperience
	LogRowEventTypeKillExperienceReward
	LogRowEventTypeCorpseCoinReward
	LogRowEventTypeLevelUp
	LogRowEventTypeAggroClear
	LogRowEventTypeYouSlain
	LogRowEventTypeSlainBy
	LogRowEventTypeZoneChange
	LogRowEventTypeLoot
	LogRowEventTypeLootResult
	LogRowEventTypeMerchantSale
	LogRowEventTypeItemDestroyed
	LogRowEventTypeTradeOffer
	LogRowEventTypeTradeComplete
	LogRowEventTypeTradeCancel
	LogRowEventTypeTradeRejected
	LogRowEventTypeWho
	LogRowEventTypeAnonymousWho
	LogRowEventTypeInventoryExport
	LogRowEventTypeSomeoneDied
	LogRowEventTypeParcelSent
	LogRowEventTypeParcelReceived
	LogRowEventTypeParcelCollected
	LogRowEventTypeFailedMelee
)

type LogLandmark struct {
	Type      LogRowEventType
	Timestamp time.Time
	Offset    int64
	Zone      string
	Level     int
}
type CharacterMetadata struct {
	CharacterName string
	ServerName    string
	Level         int
	Zone          string
	Classes       []string
	Race          string
	WhoObservedAt time.Time
}

type LogRowEvent struct {
	Session   uint64
	Timestamp time.Time
	Offset    int64
	Message   string
	Live      bool
	Type      LogRowEventType
	Data      []string
	Metadata  CharacterMetadata
}
