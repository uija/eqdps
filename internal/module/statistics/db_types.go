package statistics

import "time"

type KillType string

const (
	KillPlayer  KillType = "player"
	KillOther   KillType = "other"
	KillUnknown KillType = "unknown"
)

func (value KillType) valid() bool {
	switch value {
	case KillPlayer, KillOther, KillUnknown:
		return true
	default:
		return false
	}
}

type LootDestination string

const (
	LootInventory       LootDestination = "inventory"
	LootBank            LootDestination = "bank"
	LootTradeskillDepot LootDestination = "tradeskill_depot"
	LootCurrency        LootDestination = "currency"
	LootSold            LootDestination = "sold"
	LootUnknown         LootDestination = "unknown"
)

func (value LootDestination) valid() bool {
	switch value {
	case LootInventory, LootBank, LootTradeskillDepot, LootCurrency, LootSold, LootUnknown:
		return true
	default:
		return false
	}
}

type MoneySource string

const (
	MoneyCorpse   MoneySource = "corpse"
	MoneyLootSale MoneySource = "loot_sale"
	MoneyParcel   MoneySource = "parcel"
	MoneyTrade    MoneySource = "trade"
	MoneyMerchant MoneySource = "merchant"
)

func (value MoneySource) valid() bool {
	switch value {
	case MoneyCorpse, MoneyLootSale, MoneyParcel, MoneyTrade, MoneyMerchant:
		return true
	default:
		return false
	}
}

type ChatChannel string

const (
	ChatGuild   ChatChannel = "guild"
	ChatTell    ChatChannel = "tell"
	ChatGroup   ChatChannel = "group"
	ChatRaid    ChatChannel = "raid"
	ChatSay     ChatChannel = "say"
	ChatOOC     ChatChannel = "ooc"
	ChatAuction ChatChannel = "auction"
)

type Direction string

const (
	DirectionSent     Direction = "sent"
	DirectionReceived Direction = "received"
)

func (value Direction) valid() bool {
	return value == DirectionSent || value == DirectionReceived
}

type Kill struct {
	ID       int64
	ZoneID   int64
	MobID    int64
	KilledAt time.Time
	Type     KillType
}

type Parcel struct {
	ID               int64
	Direction        Direction
	OtherCharacter   string
	ItemID           *int64
	Quantity         int
	MoneyCopper      *int64
	SentOrReceivedAt time.Time
	CollectedAt      *time.Time
}
