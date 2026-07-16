// Command skyquestdb extracts the Plane of Sky class quest tables from EQL Wiki.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	sourceURL = "https://eqlwiki.com/Plane_of_Sky#Plane_of_Sky_Class_Quests"
)

type database struct {
	SchemaVersion  int     `json:"schema_version"`
	Source         string  `json:"source"`
	SourcePageID   int     `json:"source_page_id"`
	SourceRevision int     `json:"source_revision"`
	Classes        []class `json:"classes"`
}

type class struct {
	Name           string  `json:"name"`
	QuestNPC       string  `json:"quest_npc"`
	Source         string  `json:"source,omitempty"`
	SourcePageID   int     `json:"source_page_id,omitempty"`
	SourceRevision int     `json:"source_revision,omitempty"`
	Quests         []quest `json:"quests"`
}

type quest struct {
	Name         string        `json:"name"`
	QuestGiver   string        `json:"quest_giver"`
	Requirements []requirement `json:"requirements"`
	Rewards      []string      `json:"rewards"`
}

type requirement struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Quantity   int    `json:"quantity"`
	NoDrop     bool   `json:"no_drop,omitempty"`
	Island     int    `json:"island,omitempty"`
	DropsFrom  string `json:"drops_from,omitempty"`
	SourceHint string `json:"source_hint,omitempty"`
}

type apiResponse struct {
	Parse struct {
		PageID   int    `json:"pageid"`
		Revision int    `json:"revid"`
		Wikitext string `json:"wikitext"`
	} `json:"parse"`
}

var (
	sectionRE = regexp.MustCompile(`(?s)== Plane of Sky Class Quests ==\s*(.*?)\s*=+ Random Drop Items =+`)
	classRE   = regexp.MustCompile(`(?s)<h3>\s*\[\[([^]|]+)(?:\|[^]]+)?\]\]\s*\(([^)]+)\)\s*</h3>\s*(\{\|.*?\n\|\})`)
	linkRE    = regexp.MustCompile(`\[\[([^]|]+)(?:\|[^]]+)?\]\]`)
	rewardRE  = regexp.MustCompile(`\{\{:([^}|]+)`)
	hintRE    = regexp.MustCompile(`\(([2-8])-([^)]+)\)`)
	tagRE     = regexp.MustCompile(`<[^>]+>`)
	npcRE     = regexp.MustCompile(`(?i)Find\s+(?:\[\[)?([^\]\n]+?)(?:\]\])?\s+and Hail`)
	talkRE    = regexp.MustCompile(`(?i)Talk to\s+(?:\[\[)?([^\]\n]+?)(?:\]\])?\s+in the`)
	testRE    = regexp.MustCompile(`(?m)^==+\s*((?:\w+(?:\s+\w+)*\s+)?Test of [^=\n]+?)\s*==+\s*$`)
)

var sourceNames = map[string]string{
	"PoS":   "Protector of Sky",
	"Gorga": "Gorgalosk",
	"KoS":   "Keeper of Souls",
	"SL":    "The Spiroc Lord",
	"BZ":    "Bazzt Zzzt",
	"SotS":  "Sister of the Spire",
	"Trash": "Island trash",
	"EoV":   "Eye of Veeshan",
}

func main() {
	output := flag.String("output", "internal/skyquest/plane_of_sky_quests.json", "output JSON path")
	input := flag.String("input", "", "optional saved MediaWiki API response")
	flag.Parse()

	var response apiResponse
	var err error
	if *input != "" {
		err = decodeFile(*input, &response)
	} else {
		err = fetchPage("Plane_of_Sky", &response)
	}
	if err != nil {
		fatal(err)
	}

	db, err := extract(response)
	if err != nil {
		fatal(err)
	}
	if *input == "" {
		if err := enrichFromClassPages(&db); err != nil {
			fatal(err)
		}
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		fatal(err)
	}

	quests, items := counts(db)
	fmt.Printf("wrote %s: %d classes, %d quests, %d unique required items\n", *output, len(db.Classes), quests, items)
}

func fetchPage(page string, target *apiResponse) error {
	client := &http.Client{Timeout: 30 * time.Second}
	url := "https://eqlwiki.com/api.php?action=parse&page=" + page + "&prop=wikitext%7Crevid&format=json&formatversion=2"
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("EQL Wiki returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func enrichFromClassPages(db *database) error {
	for index := range db.Classes {
		entry := &db.Classes[index]
		page := strings.ReplaceAll(entry.Name, " ", "_") + "_Plane_of_Sky_Tests"
		var response apiResponse
		if err := fetchPage(page, &response); err != nil || response.Parse.PageID == 0 || strings.HasPrefix(response.Parse.Wikitext, "#Redirect") {
			continue
		}
		npc := ""
		if match := npcRE.FindStringSubmatch(response.Parse.Wikitext); match != nil {
			npc = clean(match[1])
		}
		if npc == "" {
			if match := talkRE.FindStringSubmatch(response.Parse.Wikitext); match != nil {
				npc = clean(match[1])
			}
		}
		if npc == "" {
			continue
		}
		updated := 0
		source := response.Parse.Wikitext
		headings := testRE.FindAllStringSubmatchIndex(source, -1)
		for _, match := range headings {
			name := clean(source[match[2]:match[3]])
			for questIndex := range entry.Quests {
				if sameTest(entry.Quests[questIndex].Name, name) {
					entry.Quests[questIndex].Name = name
					entry.Quests[questIndex].QuestGiver = npc
					updated++
					break
				}
			}
		}
		if updated != len(entry.Quests) {
			return fmt.Errorf("%s class page matched %d of %d quests", entry.Name, updated, len(entry.Quests))
		}
		entry.QuestNPC = npc
		entry.Source = "https://eqlwiki.com/" + page
		entry.SourcePageID = response.Parse.PageID
		entry.SourceRevision = response.Parse.Revision
	}
	return nil
}

func sameTest(left, right string) bool {
	normalize := func(value string) string {
		var words []string
		for _, word := range strings.Fields(strings.ToLower(value)) {
			if word != "test" && word != "of" {
				words = append(words, word)
			}
		}
		return strings.Join(words, " ")
	}
	return normalize(left) == normalize(right)
}

func decodeFile(path string, target *apiResponse) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(target)
}

func extract(response apiResponse) (database, error) {
	match := sectionRE.FindStringSubmatch(response.Parse.Wikitext)
	if match == nil {
		return database{}, fmt.Errorf("Plane of Sky class quest section not found")
	}

	db := database{SchemaVersion: 1, Source: sourceURL, SourcePageID: response.Parse.PageID, SourceRevision: response.Parse.Revision}
	for _, classMatch := range classRE.FindAllStringSubmatch(match[1], -1) {
		entry := class{Name: clean(classMatch[1]), QuestNPC: clean(classMatch[2])}
		for _, rawRow := range strings.Split(classMatch[3], "\n|-") {
			cells := tableCells(rawRow)
			if len(cells) < 6 || strings.EqualFold(clean(cells[0]), "Quest") {
				continue
			}
			q := quest{
				Name:         clean(cells[0]),
				QuestGiver:   clean(cells[1]),
				Requirements: append(requirements(cells[3], "rune"), requirements(cells[4], "item")...),
				Rewards:      rewards(strings.Join(cells[5:], "\n")),
			}
			if q.Name == "" || q.QuestGiver == "" || len(q.Requirements) == 0 || len(q.Rewards) == 0 {
				return database{}, fmt.Errorf("incomplete quest extracted for %s: %#v", entry.Name, q)
			}
			entry.Quests = append(entry.Quests, q)
		}
		if len(entry.Quests) == 0 {
			return database{}, fmt.Errorf("no quests extracted for %s", entry.Name)
		}
		db.Classes = append(db.Classes, entry)
	}
	if len(db.Classes) != 16 {
		return database{}, fmt.Errorf("expected 16 classes, extracted %d", len(db.Classes))
	}
	return db, nil
}

func tableCells(row string) []string {
	lines := strings.Split(row, "\n")
	var cells []string
	for _, line := range lines {
		if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "|}") && !strings.HasPrefix(line, "|-") {
			value := strings.TrimSpace(strings.TrimLeft(line, "|"))
			if value == "" {
				continue
			}
			cells = append(cells, value)
			continue
		}
		if len(cells) > 0 {
			cells[len(cells)-1] += "\n" + line
		}
	}
	return cells
}

func requirements(cell, kind string) []requirement {
	var result []requirement
	for _, line := range strings.Split(cell, "\n") {
		link := linkRE.FindStringSubmatch(line)
		if link == nil {
			continue
		}
		item := requirement{Name: clean(link[1]), Kind: kind, Quantity: 1, NoDrop: strings.Contains(line, "SkyNoDrop")}
		if hint := hintRE.FindStringSubmatch(line); hint != nil {
			item.Island, _ = strconv.Atoi(hint[1])
			item.SourceHint = hint[0]
			item.DropsFrom = sourceNames[hint[2]]
		}
		result = append(result, item)
	}
	return result
}

func rewards(cell string) []string {
	var result []string
	for _, match := range rewardRE.FindAllStringSubmatch(cell, -1) {
		result = append(result, clean(match[1]))
	}
	return result
}

func clean(value string) string {
	value = tagRE.ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, "'''", "")
	value = strings.ReplaceAll(value, "''", "")
	return strings.Join(strings.Fields(value), " ")
}

func counts(db database) (int, int) {
	quests := 0
	items := make(map[string]struct{})
	for _, class := range db.Classes {
		quests += len(class.Quests)
		for _, quest := range class.Quests {
			for _, item := range quest.Requirements {
				items[item.Name] = struct{}{}
			}
		}
	}
	return quests, len(items)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
