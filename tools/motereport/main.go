package main

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const idleCutoff = 5 * time.Minute
const timestampLayout = "Mon Jan 02 15:04:05 2006"

var (
	logLineRE    = regexp.MustCompile(`^\[([^]]+)\] (.*)$`)
	zoneChangeRE = regexp.MustCompile(`^You have entered (.+)\.$`)
	moteNameRE   = regexp.MustCompile(`(?i)^(?:([0-9]+)|a|an) Mote of ([A-Za-z]+) Potential$`)
	lootREs      = []*regexp.Regexp{
		regexp.MustCompile(`^--You have looted ((?:a|an|[0-9]+) .+) from .+'s corpse\.--$`),
		regexp.MustCompile(`^You looted ((?:a|an|[0-9]+) .+) from .+'s corpse (?:and sold it for .+\.|and stored it in .+|to create .+)$`),
	}
	damageREs = []*regexp.Regexp{
		regexp.MustCompile(`^(.+?) (backstab|backstabs|bash|bashes|bite|bites|cleave|cleaves|claw|claws|crush|crushes|frenzy on|frenzies on|hit|hits|kick|kicks|maul|mauls|pierce|pierces|punch|punches|reave|reaves|shoot|shoots|slash|slashes|slice|slices|smash|smashes|smite|smites|strike|strikes) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)(?: by ([^.]+))?\.(?: \(([^)]+)\))?$`),
		regexp.MustCompile(`^(.+?) is .+? by YOUR (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)\.(?: \(([^)]+)\))?$`),
		regexp.MustCompile("^(.+?) (?:is|are) .+? by (.+)(?:'s|`s) (.+?) for ([0-9]+) points? of ((?:[A-Za-z-]+ )?damage)[.!](?: \\(([^)]+)\\))?$"),
		regexp.MustCompile(`^(.+?) has taken ([0-9]+) damage from your (.+?)\.(?: \(([^)]+)\))?$`),
		regexp.MustCompile(`^(.+?) (?:has|have) taken ([0-9]+) damage from (.+?) by ([^.]+)\.(?: \(([^)]+)\))?$`),
	}
)

type session struct {
	Start       time.Time
	End         time.Time
	DamageTimes []time.Time
	Motes       int
	AboveMajor  int
	Active      time.Duration
	Rate        float64
}

type report struct {
	Zone              string
	Logfile           string
	Sessions          []session
	TotalActive       time.Duration
	TotalActiveHours  float64
	TotalMotes        int
	TotalAboveMajor   int
	TotalRate         float64
	AboveMajorPercent float64
}

func isDamage(message string) bool {
	for _, expression := range damageREs {
		if expression.MatchString(message) {
			return true
		}
	}
	return false
}

func isAboveMajor(tier string) bool {
	switch strings.ToLower(tier) {
	case "greater", "superior", "ascendant", "grand", "infinite":
		return true
	default:
		return false
	}
}

func moteFromMessage(message string) (quantity int, aboveMajor bool, ok bool) {
	item := ""
	for _, expression := range lootREs {
		matches := expression.FindStringSubmatch(message)
		if matches != nil {
			item = matches[1]
			break
		}
	}
	if item == "" {
		return 0, false, false
	}
	matches := moteNameRE.FindStringSubmatch(strings.TrimSpace(item))
	if matches == nil {
		return 0, false, false
	}
	quantity = 1
	if matches[1] != "" {
		if _, err := fmt.Sscanf(matches[1], "%d", &quantity); err != nil {
			return 0, false, false
		}
	}
	return quantity, isAboveMajor(matches[2]), true
}

func activeTime(s session) time.Duration {
	if len(s.DamageTimes) == 0 {
		return 0
	}
	var active time.Duration
	for index := 1; index < len(s.DamageTimes); index++ {
		gap := s.DamageTimes[index].Sub(s.DamageTimes[index-1])
		if gap < 0 {
			continue
		}
		active += min(gap, idleCutoff)
	}
	if !s.End.IsZero() {
		tail := s.End.Sub(s.DamageTimes[len(s.DamageTimes)-1])
		if tail > 0 {
			active += min(tail, idleCutoff)
		}
	}
	return active
}

func analyse(logfile, zone string) (report, error) {
	file, err := os.Open(logfile)
	if err != nil {
		return report{}, fmt.Errorf("open logfile: %w", err)
	}
	defer file.Close()

	result := report{Zone: zone, Logfile: logfile}
	var current *session
	finishCurrent := func(end time.Time) {
		if current == nil {
			return
		}
		if !end.IsZero() {
			current.End = end
		}
		current.Active = activeTime(*current)
		if current.Active > 0 {
			current.Rate = float64(current.Motes) / current.Active.Hours()
		}
		result.Sessions = append(result.Sessions, *current)
		result.TotalActive += current.Active
		result.TotalMotes += current.Motes
		result.TotalAboveMajor += current.AboveMajor
		current = nil
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		matches := logLineRE.FindStringSubmatch(strings.TrimSuffix(scanner.Text(), "\r"))
		if matches == nil {
			continue
		}
		timestamp, err := time.Parse(timestampLayout, matches[1])
		if err != nil {
			continue
		}
		message := matches[2]
		if zoneMatch := zoneChangeRE.FindStringSubmatch(message); zoneMatch != nil {
			finishCurrent(timestamp)
			if zoneMatch[1] == zone {
				current = &session{Start: timestamp, End: timestamp}
			}
			continue
		}
		if current == nil {
			continue
		}
		current.End = timestamp
		if isDamage(message) {
			current.DamageTimes = append(current.DamageTimes, timestamp)
		}
		if quantity, aboveMajor, ok := moteFromMessage(message); ok {
			current.Motes += quantity
			if aboveMajor {
				current.AboveMajor += quantity
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return report{}, fmt.Errorf("read logfile: %w", err)
	}
	finishCurrent(time.Time{})

	result.TotalActiveHours = result.TotalActive.Hours()
	if result.TotalActiveHours > 0 {
		result.TotalRate = float64(result.TotalMotes) / result.TotalActiveHours
	}
	if result.TotalMotes > 0 {
		result.AboveMajorPercent = 100 * float64(result.TotalAboveMajor) / float64(result.TotalMotes)
	}
	return result, nil
}

func safeFilename(value string) string {
	var result strings.Builder
	underscore := false
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			underscore = false
		} else if !underscore && result.Len() > 0 {
			result.WriteByte('_')
			underscore = true
		}
	}
	return strings.Trim(result.String(), "_")
}

func outputPath(logfile, zone string) string {
	base := filepath.Base(logfile)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(logfile), base+"_"+safeFilename(zone)+"_motes.html")
}

var page = template.Must(template.New("report").Funcs(template.FuncMap{
	"dateTime": func(value time.Time) string { return value.Format("Jan 2, 2006 15:04") },
	"hours":    func(value time.Duration) float64 { return value.Hours() },
	"number":   func(value float64) string { return fmt.Sprintf("%.2f", value) },
	"addOne":   func(value int) int { return value + 1 },
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Zone}} Mote Sessions</title>
  <style>
    :root { color-scheme: dark; font-family: system-ui, sans-serif; background: #111416; color: #e4e7e9; }
    body { max-width: 1050px; margin: 0 auto; padding: 32px 20px 48px; }
    h1 { margin-bottom: 8px; font-size: 1.7rem; }
    .subtitle, .method { color: #aeb5b9; }
    .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin: 24px 0; }
    .summary div { padding: 16px; border: 1px solid #343b3f; border-radius: 7px; background: #1b2023; }
    .summary strong { display: block; font-size: 1.55rem; color: #e7bd55; }
    .summary span { color: #aeb5b9; font-size: .9rem; }
    .table-wrap { overflow-x: auto; border: 1px solid #343b3f; border-radius: 7px; }
    table { width: 100%; border-collapse: collapse; font-variant-numeric: tabular-nums; }
    th, td { padding: 10px 14px; border-bottom: 1px solid #2b3134; text-align: right; white-space: nowrap; }
    th { position: sticky; top: 0; background: #252b2f; color: #c8ced1; font-size: .82rem; text-transform: uppercase; letter-spacing: .04em; }
    th:nth-child(2), td:nth-child(2) { text-align: left; }
    tbody tr:nth-child(even) { background: #181d20; }
    tbody tr:hover { background: #242b2f; }
    tfoot { background: #252b2f; color: #fff; font-weight: 700; }
    .higher { color: #73c991; font-weight: 600; }
    .method { margin-top: 18px; line-height: 1.5; font-size: .92rem; }
  </style>
</head>
<body>
  <h1>{{.Zone}}</h1>
  <div class="subtitle">Mote drops split by session</div>
  <section class="summary" aria-label="Summary">
    <div><strong>{{.TotalMotes}}</strong><span>All Motes</span></div>
    <div><strong>{{.TotalAboveMajor}}</strong><span>Above Major ({{number .AboveMajorPercent}}%)</span></div>
    <div><strong>{{number .TotalActiveHours}} h</strong><span>Active time</span></div>
    <div><strong>{{number .TotalRate}}</strong><span>Motes per active hour</span></div>
  </section>
  <div class="table-wrap">
    <table>
      <thead><tr><th>#</th><th>Session</th><th>Active hours</th><th>All Motes</th><th>&gt; Major</th><th>Motes/hour</th></tr></thead>
      <tbody>
        {{range $index, $session := .Sessions}}<tr><td>{{addOne $index}}</td><td>{{dateTime $session.Start}}–{{dateTime $session.End}}</td><td>{{number (hours $session.Active)}}</td><td>{{$session.Motes}}</td><td class="{{if $session.AboveMajor}}higher{{end}}">{{$session.AboveMajor}}</td><td>{{number $session.Rate}}</td></tr>
        {{end}}
      </tbody>
      <tfoot><tr><td></td><td>Total</td><td>{{number .TotalActiveHours}}</td><td>{{.TotalMotes}}</td><td>{{.TotalAboveMajor}}</td><td>{{number .TotalRate}}</td></tr></tfoot>
    </table>
  </div>
  <p class="method">Active time is based on parsed combat-damage events. Gaps longer than five minutes are capped at five minutes to exclude extended idle time. “&gt; Major” includes Greater, Superior, Ascendant, Grand, and Infinite Motes.</p>
</body>
</html>`))

func run(logfile, zone string) (string, error) {
	report, err := analyse(logfile, zone)
	if err != nil {
		return "", err
	}
	path := outputPath(logfile, zone)
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create report: %w", err)
	}
	if err := page.Execute(file, report); err != nil {
		file.Close()
		return "", fmt.Errorf("render report: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close report: %w", err)
	}
	return path, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: motereport <logfile> <full-zone-name>")
		os.Exit(2)
	}
	path, err := run(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "motereport: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(path)
}
