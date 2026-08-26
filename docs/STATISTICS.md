# Statistics Design

The statistics module is not updated in real time. The user explicitly starts
parsing, and the module continues from its own stored logfile offset.

Individual observations should be stored wherever practical. Overall totals
can then be calculated from those observations without reparsing the logfile
when new views or aggregations are added.

Existing parser event types are defined in
`internal/data/logrowevent.go`. Their patterns are in
`internal/eqlog/patterns.go`.

## Implementation roadmap

1. Define the schema:
   - decide on tables, columns, relationships, constraints, and indexes;
   - decide which observations are stored individually and which values are
     calculated;
   - delete and recreate the development database or introduce a schema
     version because the existing schema is incompatible.
2. Add the missing parser event types and patterns:
   - guild chat sent and received;
   - tells sent and received;
   - parcels sent, received, and collected;
   - captured denominations for corpse-money events.
3. Implement `CREATE TABLE IF NOT EXISTS` for every table:
   - add indexes for timestamps, zone IDs, mob IDs, item IDs, and names;
   - enable foreign-key enforcement.
4. Implement the database functions needed during parsing:
   - find or create zones, mobs, and items;
   - insert visits, kills, loot, money, XP, levels, chat, and parcels;
   - find and update received parcels when they are collected;
   - read and update the logfile offset.
5. Implement efficient importing:
   - use transactions and prepared statements instead of separately committed
     writes for every event;
   - commit large imports in batches;
   - update the logfile offset in the same transaction as the corresponding
     imported events so a crash cannot lose or duplicate committed data.
6. Implement the state needed while parsing:
   - the current zone;
   - recent kills used to associate loot with a kill;
   - pending received parcels used to associate collection messages;
   - player/NPC knowledge needed to classify tells and kills.
7. Implement the manually started statistics parser:
   - use a separate parser instance;
   - begin at the stored offset;
   - show progress;
   - support cancellation;
   - report parsing and database errors;
   - advance the offset only for committed batches.
8. Verify imported data:
   - test new parser patterns with real log lines;
   - test database relationships and totals;
   - parse a small known logfile sample and compare its results manually.
9. Decide which statistics and relationships the UI should display.
10. Implement database queries for those views.
11. Implement the statistics UI.

## Database structure

Suggested tables:

- `zones`
- `zone_visits`
- `mobs`
- `kills`
- `items`
- `loot`
- `money`
- `experience`
- `levels`
- `chat`
- `parcels`
- `log_state`

The `items` table should not belong directly to a mob. The same item can drop
from multiple mobs; the `loot` table supplies that relationship.

## Event-to-database mapping

| Statistic | Parser event | Database action |
| --- | --- | --- |
| Zones visited | `LogRowEventTypeZoneChange` | Find or create the zone and insert one `zone_visits` row. |
| Visit count | Derived from `zone_visits` | Count visits grouped by zone. |
| Mobs killed | `LogRowEventTypeYouSlain` and applicable `LogRowEventTypeSlainBy` events | Find or create the mob and insert one `kills` row. |
| Player deaths | `LogRowEventTypeSlainBy` with `You` as the target | Insert a death record, either in `kills` or a separate `deaths` table. |
| Corpse money | `LogRowEventTypeCorpseCoinReward` | Insert one positive `money` row. |
| Auto-sold loot money | `LogRowEventTypeLootResult` containing `and sold it for ...` | Insert the loot observation and a positive `money` row. |
| XP received | `LogRowEventTypeExperience` | Insert one `experience` row containing the percentage. |
| XP reward without a percentage | `LogRowEventTypeKillExperienceReward` | Optionally insert an event with an unknown amount. |
| Levels gained | `LogRowEventTypeLevelUp` | Insert one `levels` row. |
| Items looted | `LogRowEventTypeLoot` and `LogRowEventTypeLootResult` | Find or create the item and insert one `loot` row. |
| Motes collected | The existing loot events | Derive by filtering item names beginning with `Mote of`. |
| Guild messages sent | New `LogRowEventTypeGuildChatSent` | Insert one `chat` row. |
| Guild messages received | New `LogRowEventTypeGuildChatReceived` | Insert one `chat` row. |
| Tells sent | New `LogRowEventTypeTellSent` | Insert one `chat` row. |
| Tells received | New `LogRowEventTypeTellReceived` | Insert one `chat` row. |
| Parcels sent | New `LogRowEventTypeParcelSent` | Insert one `parcels` row. |
| Parcel notification received | New `LogRowEventTypeParcelReceived` | Insert one `parcels` row. |
| Parcel collected | New `LogRowEventTypeParcelCollected` | Update the matching received parcel with `collected_at`. |

## Tables

### `zones`

Stores one row per normalized zone name.

```text
id
name
```

### `zone_visits`

Stores one row every time the player enters a zone.

```text
id
zone_id
entered_at
raw_zone_name
```

`raw_zone_name` retains difficulty and instance information even when
`zone_id` refers to the normalized base zone.

### `mobs`

Stores one mob name per zone.

```text
id
zone_id
name
```

### `kills`

```text
id
zone_id
mob_id
killed_at
kill_type
```

`kill_type` can distinguish between a mob killed by the player, a mob killed
by another player, and a death with an unknown killer. Only sufficiently
confirmed kills should enter the denominator used for estimated loot chances.

### `items`

```text
id
name
```

`name` is normalized, so upgrade variants share one item row. The exact name
observed in an individual loot message is stored on that loot row.

### `loot`

```text
id
zone_id
mob_id
kill_id       nullable
item_id
raw_item_name
quantity
looted_at
destination
sale_value
```

Possible destinations include:

- inventory
- bank
- tradeskill depot
- currency
- sold
- unknown

This supports:

- items observed from each mob;
- mobs that supplied a particular item;
- total quantities collected;
- estimated chance per confirmed kill;
- Mote totals by tier;
- money generated by automatically sold loot.

The calculated chance is an observed personal-loot chance, not necessarily the
mob's actual server-side drop chance.

### `money`

Store every amount converted to copper.

```text
id
zone_id
received_at
amount_copper
source
```

Possible sources include `corpse`, `loot_sale`, `parcel`, and later `trade` or
`merchant`. Dashboard queries can convert the total back into platinum, gold,
silver, and copper.

### `experience`

```text
id
zone_id
received_at
percent
```

Summing this produces percentage points earned. A total of `235.4` represents
approximately 2.354 levels' worth of experience; it is not an absolute EQ XP
point value.

### `levels`

```text
id
zone_id
reached_at
level
```

### `chat`

The statistics described do not require storing message contents.

```text
id
sent_at
channel
direction
other_character
```

Initial channels are `guild` and `tell`. Group, raid, say, OOC, auction, and
other channels can be added later without changing the table.

NPC messages also use the `tells you` format. Known parcel and system patterns
must be matched before the generic incoming-tell pattern. Other NPC tells may
still be counted unless player/NPC classification is added.

### `parcels`

```text
id
direction
other_character
item_id                    nullable
quantity
money_copper               nullable
sent_or_received_at
collected_at               nullable
```

An incoming delivery notification creates a row. The later collection message
updates that row instead of counting a second received parcel.

### `log_state`

Stores the byte offset through which the statistics parser has committed data.
The offset should be advanced in the same transaction as the imported events.

```text
id
byte_offset
```

## New parser patterns

### Guild chat

```text
You say to your guild, '...'
Karthar tells the guild, '...'
```

These produce `LogRowEventTypeGuildChatSent` and
`LogRowEventTypeGuildChatReceived` respectively.

### Tells

```text
You told Karthar, '...'
Karthar tells you, '...'
```

These produce `LogRowEventTypeTellSent` and
`LogRowEventTypeTellReceived`.

### Incoming parcels

```text
You have received a new parcel delivery containing 1 Fishbone Earring from Sonar!
```

This produces `LogRowEventTypeParcelReceived` with the quantity, item, and
sender.

### Outgoing parcels

```text
Garu Nokel told you, 'I will deliver the stack of 100 Bone Chips to Moth as soon as possible!'
Garu Nokel told you, 'I will deliver the Efreeti Scimitar to Slacks as soon as possible!'
Garu Nokel told you, 'I will deliver the Money (100p) to Oakpine as soon as possible!'
```

These produce `LogRowEventTypeParcelSent` with the quantity and item or money
amount, plus the recipient.

### Parcel collection

```text
Garu Nokel hands you the Fishbone Earring that was sent from Sonar.
```

This produces `LogRowEventTypeParcelCollected` and updates the corresponding
incoming parcel.

### Corpse money

The existing `LogRowEventTypeCorpseCoinReward` pattern recognizes messages such
as:

```text
You receive 7 platinum, 7 gold, 8 silver and 7 copper from the corpse.
```

It currently captures no denominations. The pattern needs to provide the coin
text to the event, or the statistics importer needs a secondary coin parser,
before it can populate `money.amount_copper`.
