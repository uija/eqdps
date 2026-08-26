package statistics

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqlog"
)

const (
	killConfirmationWindow = 2 * time.Minute
	coinConfirmationWindow = 2 * time.Second
	campCompletionTime     = 30 * time.Second
	loginSequenceTimeout   = 5 * time.Minute

	loginMessage          = "Welcome to EverQuest Legends!"
	loginAdventureMessage = "You are not currently assigned to an adventure."
	campStartMessage      = "It will take you about 30 seconds to prepare your camp."
	campAbandonMessage    = "You abandon your preparations to camp."
)

var (
	coinAmountRE      = regexp.MustCompile(`(?i)([0-9]+)\s+(platinum|gold|silver|copper)`)
	shortCoinAmountRE = regexp.MustCompile(`(?i)([0-9]+)\s*([pgsc])`)
	parcelMoneyRE     = regexp.MustCompile(`(?i)^Money \(([^)]+)\)$`)
	guildSentRE       = regexp.MustCompile(`^You say to your guild, '.*'$`)
	guildReceivedRE   = regexp.MustCompile(`^(.+?) tells the guild, '.*'$`)
	tellSentRE        = regexp.MustCompile(`^You told (.+?), '.*'$`)
	tellReceivedRE    = regexp.MustCompile(`^(.+?) tells you, '.*'$`)
)

type pendingStatisticsKill struct {
	zoneID   int64
	mobName  string
	killedAt time.Time
}

type unsupportedObservationError struct {
	message string
}

func (e *unsupportedObservationError) Error() string {
	return e.message
}

func unsupportedObservation(format string, values ...any) error {
	return &unsupportedObservationError{message: fmt.Sprintf(format, values...)}
}

func (m *Module) OnLogRow(e *data.LogRowEvent) error {
	if e == nil || m.activeImport == nil {
		return nil
	}
	previousTimestamp := m.lastImportRow
	defer func() {
		if e.Timestamp.After(m.lastImportRow) {
			m.lastImportRow = e.Timestamp
		}
	}()
	if err := m.updateSessionState(e, previousTimestamp); err != nil {
		return err
	}

	m.pendingKills = slices.DeleteFunc(m.pendingKills, func(kill pendingStatisticsKill) bool {
		age := e.Timestamp.Sub(kill.killedAt)
		return age >= 0 && age > killConfirmationWindow
	})

	switch e.Type {
	case data.LogRowEventTypeZoneChange:
		if len(e.Data) < 2 {
			return unsupportedObservation("statistics zone event has no zone name")
		}
		rawName := strings.TrimSpace(e.Data[1])
		if rawName == "" {
			return unsupportedObservation("statistics zone event has an empty zone name")
		}
		zoneID, err := m.activeImport.GetOrCreateZone(normalizeZoneName(rawName))
		if err != nil {
			return err
		}
		if err := m.closeCurrentZoneVisit(e.Timestamp); err != nil {
			return err
		}
		visitID, err := m.activeImport.InsertZoneVisit(zoneID, rawName, e.Timestamp)
		if err != nil {
			return err
		}
		m.currentZone = zoneID
		m.currentVisit = visitID
		m.currentVisitAt = e.Timestamp
		m.pendingCamp = time.Time{}
		m.loginSequence = time.Time{}
		m.preLoginRow = time.Time{}
		m.pendingKills = m.pendingKills[:0]
		m.clearCoinReward()

	case data.LogRowEventTypeYouSlain:
		if m.currentZone < 1 || len(e.Data) < 2 {
			return nil
		}
		killID, err := m.insertKill(strings.TrimSpace(e.Data[1]), e.Timestamp, KillPlayer)
		if err != nil {
			return err
		}
		return m.associateRecentCoinReward(killID, e.Timestamp)

	case data.LogRowEventTypeSlainBy:
		if m.currentZone < 1 || len(e.Data) < 2 {
			return nil
		}
		target := strings.TrimSpace(e.Data[1])
		if strings.EqualFold(target, "You") {
			m.pendingKills = m.pendingKills[:0]
			m.clearCoinReward()
			if len(e.Data) < 3 {
				return nil
			}
			killer := strings.TrimSpace(e.Data[2])
			if killer == "" {
				return nil
			}
			mobID, err := m.activeImport.GetOrCreateMob(m.currentZone, killer)
			if err != nil {
				return err
			}
			_, err = m.activeImport.InsertPlayerDeath(m.currentZone, mobID, e.Timestamp)
			return err
		}
		elapsed := e.Timestamp.Sub(m.lastCoinReward)
		if !m.lastCoinReward.IsZero() && elapsed >= 0 && elapsed <= coinConfirmationWindow {
			killID, err := m.insertKill(target, e.Timestamp, KillOther)
			if err != nil {
				return err
			}
			return m.associateRecentCoinReward(killID, e.Timestamp)
		}
		m.pendingKills = append(m.pendingKills, pendingStatisticsKill{
			zoneID:   m.currentZone,
			mobName:  target,
			killedAt: e.Timestamp,
		})

	case data.LogRowEventTypeLoot, data.LogRowEventTypeLootResult:
		return m.importLoot(e)

	case data.LogRowEventTypeCorpseCoinReward:
		amount, err := parseLongCoinAmount(e.Message)
		if err != nil {
			return err
		}
		m.lastCoinReward = e.Timestamp
		m.lastCoinID, err = m.activeImport.InsertMoney(m.currentZonePointer(), e.Timestamp, amount, MoneyCorpse)
		return err

	case data.LogRowEventTypeExperience:
		if len(e.Data) < 2 {
			return unsupportedObservation("statistics experience event has no percentage")
		}
		percentage, err := strconv.ParseFloat(e.Data[1], 64)
		if err != nil {
			return unsupportedObservation("parse statistics experience %q: %v", e.Data[1], err)
		}
		_, err = m.activeImport.InsertExperience(m.currentZonePointer(), e.Timestamp, &percentage)
		return err

	case data.LogRowEventTypeKillExperienceReward:
		_, err := m.activeImport.InsertExperience(m.currentZonePointer(), e.Timestamp, nil)
		return err

	case data.LogRowEventTypeLevelUp:
		if len(e.Data) < 2 {
			return unsupportedObservation("statistics level event has no level")
		}
		level, err := strconv.Atoi(e.Data[1])
		if err != nil {
			return unsupportedObservation("parse statistics level %q: %v", e.Data[1], err)
		}
		_, err = m.activeImport.InsertLevel(m.currentZonePointer(), e.Timestamp, level)
		return err

	case data.LogRowEventTypeParcelSent:
		return m.importParcelSent(e)

	case data.LogRowEventTypeParcelReceived:
		return m.importParcelReceived(e)

	case data.LogRowEventTypeParcelCollected:
		return m.importParcelCollected(e)

	case data.LogRowEventTypeUnknown:
		return m.importChat(e)
	}
	return nil
}

func (m *Module) updateSessionState(e *data.LogRowEvent, previousTimestamp time.Time) error {
	if !m.loginSequence.IsZero() {
		elapsed := e.Timestamp.Sub(m.loginSequence)
		if elapsed < 0 || elapsed > loginSequenceTimeout {
			m.loginSequence = time.Time{}
			m.preLoginRow = time.Time{}
		}
	}
	if e.Message == loginAdventureMessage {
		m.loginSequence = e.Timestamp
		m.preLoginRow = previousTimestamp
	}

	if e.Message == campAbandonMessage {
		m.pendingCamp = time.Time{}
	} else if !m.pendingCamp.IsZero() {
		campEndedAt := m.pendingCamp.Add(campCompletionTime)
		if !e.Timestamp.Before(campEndedAt) {
			if err := m.closeCurrentZoneVisit(campEndedAt); err != nil {
				return err
			}
			m.pendingCamp = time.Time{}
		}
	}

	switch e.Message {
	case loginMessage:
		if m.currentVisit > 0 {
			leftAt := previousTimestamp
			if !m.loginSequence.IsZero() &&
				!m.preLoginRow.IsZero() &&
				(m.currentVisitAt.IsZero() || !m.preLoginRow.Before(m.currentVisitAt)) {
				leftAt = m.preLoginRow
			}
			if leftAt.IsZero() || leftAt.After(e.Timestamp) {
				leftAt = e.Timestamp
			}
			if err := m.closeCurrentZoneVisit(leftAt); err != nil {
				return err
			}
		}
		m.currentZone = 0
		m.currentVisit = 0
		m.currentVisitAt = time.Time{}
		m.pendingCamp = time.Time{}
		m.loginSequence = time.Time{}
		m.preLoginRow = time.Time{}
		m.pendingKills = m.pendingKills[:0]
		m.clearCoinReward()
	case campStartMessage:
		m.pendingCamp = e.Timestamp
	}
	return nil
}

func (m *Module) closeCurrentZoneVisit(leftAt time.Time) error {
	if m.currentVisit < 1 {
		m.currentZone = 0
		m.currentVisitAt = time.Time{}
		return nil
	}
	if !m.currentVisitAt.IsZero() && leftAt.Before(m.currentVisitAt) {
		return nil
	}
	if err := m.activeImport.CloseZoneVisit(m.currentVisit, leftAt); err != nil {
		return err
	}
	m.currentVisit = 0
	m.currentVisitAt = time.Time{}
	m.currentZone = 0
	return nil
}

func (m *Module) currentZonePointer() *int64 {
	if m.currentZone < 1 {
		return nil
	}
	zoneID := m.currentZone
	return &zoneID
}

func (m *Module) associateRecentCoinReward(killID int64, killedAt time.Time) error {
	elapsed := killedAt.Sub(m.lastCoinReward)
	if m.lastCoinReward.IsZero() || m.lastCoinID < 1 || elapsed < 0 || elapsed > coinConfirmationWindow {
		m.clearCoinReward()
		return nil
	}
	moneyID := m.lastCoinID
	m.clearCoinReward()
	return m.activeImport.AssociateMoneyWithKill(moneyID, killID)
}

func (m *Module) clearCoinReward() {
	m.lastCoinReward = time.Time{}
	m.lastCoinID = 0
}

func (m *Module) insertKill(name string, killedAt time.Time, killType KillType) (int64, error) {
	if strings.TrimSpace(name) == "" {
		return 0, unsupportedObservation("statistics kill has an empty mob name")
	}
	mobID, err := m.activeImport.GetOrCreateMob(m.currentZone, name)
	if err != nil {
		return 0, err
	}
	return m.activeImport.InsertKill(m.currentZone, mobID, killedAt, killType)
}

func (m *Module) importLoot(e *data.LogRowEvent) error {
	if m.currentZone < 1 || len(e.Data) < 3 {
		return nil
	}
	quantity, itemName, normalizedName, err := parseObservedItem(e.Data[1])
	if err != nil {
		return err
	}
	mobName := strings.TrimSpace(e.Data[2])
	if mobName == "" {
		return unsupportedObservation("statistics loot has an empty mob name")
	}
	mobID, err := m.activeImport.GetOrCreateMob(m.currentZone, mobName)
	if err != nil {
		return err
	}

	remaining := m.pendingKills[:0]
	for _, kill := range m.pendingKills {
		if kill.zoneID == m.currentZone && strings.EqualFold(kill.mobName, mobName) {
			if _, err := m.insertKill(kill.mobName, kill.killedAt, KillOther); err != nil {
				return err
			}
			continue
		}
		remaining = append(remaining, kill)
	}
	m.pendingKills = remaining

	itemID, err := m.activeImport.GetOrCreateItem(normalizedName)
	if err != nil {
		return err
	}
	var killID *int64
	if kill, found, err := m.activeImport.FindRecentKill(m.currentZone, mobID, e.Timestamp, killConfirmationWindow); err != nil {
		return err
	} else if found {
		killID = &kill.ID
	}

	destination := LootInventory
	var saleValue *int64
	if e.Type == data.LogRowEventTypeLootResult && len(e.Data) >= 4 {
		outcome := strings.ToLower(strings.TrimSpace(e.Data[3]))
		switch {
		case strings.HasPrefix(outcome, "and sold it for "):
			destination = LootSold
			value, err := parseLongCoinAmount(e.Data[3])
			if err != nil {
				return err
			}
			saleValue = &value
		case strings.Contains(outcome, "tradeskill depot"):
			destination = LootTradeskillDepot
		case strings.Contains(outcome, "currency"):
			destination = LootCurrency
		case strings.Contains(outcome, "bank"):
			destination = LootBank
		case strings.Contains(outcome, "inventory"), strings.HasPrefix(outcome, "to create "):
			destination = LootInventory
		default:
			destination = LootUnknown
		}
	}
	if _, err := m.activeImport.InsertLoot(
		m.currentZone,
		mobID,
		killID,
		itemID,
		itemName,
		quantity,
		e.Timestamp,
		destination,
		saleValue,
	); err != nil {
		return err
	}
	if saleValue != nil && *saleValue > 0 {
		_, err = m.activeImport.InsertMoney(m.currentZonePointer(), e.Timestamp, *saleValue, MoneyLootSale)
	}
	return err
}

func (m *Module) importParcelSent(e *data.LogRowEvent) error {
	if len(e.Data) < 5 {
		return unsupportedObservation("statistics sent parcel event has incomplete data")
	}
	quantity, err := parseQuantity(e.Data[2])
	if err != nil {
		return err
	}
	itemID, money, err := m.parcelContent(e.Data[3])
	if err != nil {
		return err
	}
	_, err = m.activeImport.InsertParcel(DirectionSent, e.Data[4], itemID, quantity, money, e.Timestamp)
	return err
}

func (m *Module) importParcelReceived(e *data.LogRowEvent) error {
	if len(e.Data) < 4 {
		return unsupportedObservation("statistics received parcel event has incomplete data")
	}
	quantity, err := parseQuantity(e.Data[1])
	if err != nil {
		return err
	}
	itemID, money, err := m.parcelContent(e.Data[2])
	if err != nil {
		return err
	}
	_, err = m.activeImport.InsertParcel(DirectionReceived, e.Data[3], itemID, quantity, money, e.Timestamp)
	return err
}

func (m *Module) importParcelCollected(e *data.LogRowEvent) error {
	if len(e.Data) < 5 {
		return unsupportedObservation("statistics collected parcel event has incomplete data")
	}
	quantity, err := parseQuantity(e.Data[2])
	if err != nil {
		return err
	}
	itemID, money, err := m.parcelContent(e.Data[3])
	if err != nil {
		return err
	}
	parcelID, found, err := m.activeImport.FindPendingParcel(e.Data[4], itemID, quantity, money)
	if err != nil || !found {
		return err
	}
	if err := m.activeImport.MarkParcelCollected(parcelID, e.Timestamp); err != nil {
		return err
	}
	if money != nil {
		_, err = m.activeImport.InsertMoney(m.currentZonePointer(), e.Timestamp, *money, MoneyParcel)
	}
	return err
}

func (m *Module) parcelContent(value string) (*int64, *int64, error) {
	value = strings.TrimSpace(value)
	if matches := parcelMoneyRE.FindStringSubmatch(value); matches != nil {
		amount, err := parseShortCoinAmount(matches[1])
		return nil, &amount, err
	}
	_, _, normalizedName, err := parseObservedItem(value)
	if err != nil {
		return nil, nil, err
	}
	itemID, err := m.activeImport.GetOrCreateItem(normalizedName)
	if err != nil {
		return nil, nil, err
	}
	return &itemID, nil, nil
}

func (m *Module) importChat(e *data.LogRowEvent) error {
	message := strings.TrimSpace(e.Message)
	if guildSentRE.MatchString(message) {
		_, err := m.activeImport.InsertChat(e.Timestamp, ChatGuild, DirectionSent, "")
		return err
	}
	if matches := guildReceivedRE.FindStringSubmatch(message); matches != nil {
		_, err := m.activeImport.InsertChat(e.Timestamp, ChatGuild, DirectionReceived, matches[1])
		return err
	}
	if matches := tellSentRE.FindStringSubmatch(message); matches != nil {
		_, err := m.activeImport.InsertChat(e.Timestamp, ChatTell, DirectionSent, matches[1])
		return err
	}
	if matches := tellReceivedRE.FindStringSubmatch(message); matches != nil {
		_, err := m.activeImport.InsertChat(e.Timestamp, ChatTell, DirectionReceived, matches[1])
		return err
	}
	return nil
}

func parseObservedItem(value string) (int, string, string, error) {
	quantity, normalizedName := data.NormalizeItemName(value)
	value = strings.TrimSpace(value)
	prefix, remainder, found := strings.Cut(value, " ")
	itemName := value
	if found && (strings.EqualFold(prefix, "a") || strings.EqualFold(prefix, "an")) {
		itemName = strings.TrimSpace(remainder)
	} else if found {
		if parsed, err := strconv.Atoi(prefix); err == nil && parsed > 0 {
			itemName = strings.TrimSpace(remainder)
		}
	}
	if quantity < 1 || itemName == "" || normalizedName == "" {
		return 0, "", "", unsupportedObservation("parse statistics item %q", value)
	}
	return quantity, itemName, normalizedName, nil
}

func parseQuantity(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 1, nil
	}
	quantity, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || quantity < 1 {
		return 0, unsupportedObservation("parse statistics quantity %q", value)
	}
	return quantity, nil
}

func parseLongCoinAmount(value string) (int64, error) {
	if strings.Contains(strings.ToLower(value), "for free") {
		return 0, nil
	}
	return parseCoinMatches(value, coinAmountRE, map[string]int64{
		"platinum": 1000,
		"gold":     100,
		"silver":   10,
		"copper":   1,
	})
}

func parseShortCoinAmount(value string) (int64, error) {
	return parseCoinMatches(value, shortCoinAmountRE, map[string]int64{
		"p": 1000,
		"g": 100,
		"s": 10,
		"c": 1,
	})
}

func parseCoinMatches(value string, expression *regexp.Regexp, multipliers map[string]int64) (int64, error) {
	matches := expression.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return 0, unsupportedObservation("parse statistics money %q", value)
	}
	var total int64
	for _, match := range matches {
		amount, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return 0, unsupportedObservation("parse statistics money %q: %v", value, err)
		}
		total += amount * multipliers[strings.ToLower(match[2])]
	}
	return total, nil
}
func (m *Module) RunImport() {
	if m.logPath == "" {
		return
	}
	if m.db == nil {
		return
	}
	if !m.importRunning.CompareAndSwap(false, true) {
		return
	}
	go func() {
		var parser *eqlog.Parser
		defer func() {
			m.activeImport = nil
			m.importRunning.Store(false)
			if parser != nil {
				parser.Close()
			}
			m.invalidateFunc()
		}()
		stats, err := os.Stat(m.logPath)
		if err != nil {
			log.Printf("Unable to get stat on log. %v", err)
			return
		}
		m.lastLogfileOffset = stats.Size()
		offset, err := GetLogOffset(m.db)
		if err != nil {
			log.Printf("Unable to get log offset from db. %v", err)
			return
		}
		m.lastKnownOffset = offset
		if offset >= stats.Size() {
			log.Printf("We already at the end of the logfile")
			return
		}

		currentVisit, currentZone, currentVisitAt, _, err := GetOpenZoneVisit(m.db)
		if err != nil {
			log.Printf("Unable to restore open statistics zone visit. %v", err)
			return
		}
		lastTimestamp, err := GetLastLogTimestamp(m.db)
		if err != nil {
			log.Printf("Unable to restore last statistics timestamp. %v", err)
			return
		}

		parser = eqlog.NewParser(2)
		if err := parser.Open(m.logPath); err != nil {
			log.Printf("Unable to open statistics parser. %v", err)
			return
		}

		activeImport, err := BeginImport(m.db)
		if err != nil {
			log.Printf("Unable to begin statistics import. %v", err)
			return
		}
		defer activeImport.Rollback()
		m.activeImport = activeImport
		m.currentZone = currentZone
		m.currentVisit = currentVisit
		m.currentVisitAt = currentVisitAt
		m.lastImportRow = lastTimestamp
		m.pendingKills = m.pendingKills[:0]
		m.clearCoinReward()

		finalOffset := offset
		var finalTimestamp time.Time
		var rowErr error
		replayErr := parser.Replay(
			eqlog.Loopback{ByteOffset: offset, IncludeUnknown: true},
			func(event *data.LogRowEvent) {
				if rowErr != nil {
					return
				}
				if event.Timestamp.After(finalTimestamp) {
					finalTimestamp = event.Timestamp
				}
				if err := m.OnLogRow(event); err != nil {
					var unsupported *unsupportedObservationError
					if errors.As(err, &unsupported) {
						log.Printf("Skipping unsupported statistics observation: %v", err)
						return
					}
					rowErr = err
					m.ctx.ParserOnReplayProgress(eqlog.ReplayProgress{Bytes: 0, Total: 0, Lines: 0})
					parser.Close()
				}
			},
			func(progress eqlog.ReplayProgress) {
				if progress.Bytes > finalOffset {
					finalOffset = progress.Bytes
				}
				m.ctx.ParserOnReplayProgress(progress)
			},
		)
		if rowErr != nil {
			log.Printf("Unable to import statistics row. %v", rowErr)
			return
		}
		if replayErr != nil {
			log.Printf("Unable to replay logfile for statistics. %v", replayErr)
			return
		}
		if err := activeImport.Commit(finalOffset, finalTimestamp); err != nil {
			log.Printf("Unable to commit statistics import. %v", err)
			return
		}
		m.importDone <- struct{}{}

		m.mu.Lock()
		m.lastKnownOffset = finalOffset
		m.mu.Unlock()
		log.Printf("Done with statistics import")
	}()
}
