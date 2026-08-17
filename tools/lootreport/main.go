package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type lootData struct {
	Mobs map[string]mobLoot `json:"Mobs"`
}

type mobLoot struct {
	Items map[string]int `json:"Items"`
}

type row struct {
	Mob   string
	Item  string
	Count int
	First bool
	Span  int
	Group int
}

type pageData struct {
	Rows       []row
	MobCount   int
	ItemCount  int
	TotalDrops int
}

func displayItemName(name string) string {
	for _, prefix := range []string{"a ", "an "} {
		if len(name) >= len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
			return name[len(prefix):]
		}
	}
	return name
}

func isBoss(name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(first) || strings.EqualFold(name, "the Hand of Veeshan")
}

const page = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Plane of Sky Loot</title>
  <style>
    :root {
      color-scheme: dark;
      --background: #121416;
      --panel: #1f2225;
      --panel-alt: #181b1e;
      --border: #363b40;
      --text: #e1e2de;
      --muted: #969a97;
      --accent: #be9b4a;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--background);
      color: var(--text);
      font: 15px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    main {
      width: min(1100px, calc(100% - 32px));
      margin: 32px auto 64px;
    }
    h1 {
      margin: 0 0 6px;
      font-size: 28px;
      font-weight: 650;
    }
    .subtitle { margin: 0 0 24px; color: var(--muted); }
    .summary {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 12px;
      margin-bottom: 24px;
    }
    .summary div {
      padding: 14px 16px;
      border: 1px solid var(--border);
      border-radius: 7px;
      background: var(--panel);
    }
    .summary strong { display: block; color: var(--accent); font-size: 21px; }
    .summary span { color: var(--muted); font-size: 13px; }
    .table-wrap {
      overflow-x: auto;
      border: 1px solid var(--border);
      border-radius: 7px;
    }
    table { width: 100%; border-collapse: collapse; }
    thead th {
      position: sticky;
      top: 0;
      z-index: 1;
      padding: 11px 14px;
      background: #272b2f;
      color: var(--muted);
      text-align: left;
      font-size: 12px;
      letter-spacing: .06em;
      text-transform: uppercase;
    }
    tbody th, tbody td {
      padding: 8px 14px;
      border-top: 1px solid var(--border);
      vertical-align: top;
    }
    tbody th {
      width: 34%;
      color: var(--accent);
      text-align: left;
      font-weight: 650;
    }
    tbody tr.group-0 { background: var(--panel-alt); }
    tbody tr.group-1 { background: var(--panel); }
    td.count { width: 90px; text-align: right; font-variant-numeric: tabular-nums; }
    @media (max-width: 650px) {
      main { width: min(100% - 16px, 1100px); margin-top: 16px; }
      .summary { grid-template-columns: 1fr; }
      tbody th { width: 42%; }
    }
    @media print {
      :root { color-scheme: light; }
      body { background: white; color: black; }
      main { width: 100%; margin: 0; }
      .summary div, .table-wrap { border-color: #aaa; }
      thead th { position: static; background: #ddd; color: black; }
      tbody tr.group-0, tbody tr.group-1 { background: white; }
      tbody th { color: black; }
    }
  </style>
</head>
<body>
  <main>
    <h1>Plane of Sky Loot</h1>
    <p class="subtitle">Static report generated from loot.json</p>
    <section class="summary" aria-label="Summary">
      <div><strong>{{.MobCount}}</strong><span>Mobs</span></div>
      <div><strong>{{.ItemCount}}</strong><span>Mob/item entries</span></div>
      <div><strong>{{.TotalDrops}}</strong><span>Total drops recorded</span></div>
    </section>
    <div class="table-wrap">
      <table>
        <thead><tr><th>Mob</th><th>Item</th><th style="text-align:right">Count</th></tr></thead>
        <tbody>
          {{range .Rows}}<tr class="group-{{.Group}}">{{if .First}}<th scope="rowgroup" rowspan="{{.Span}}">{{.Mob}}</th>{{end}}<td>{{.Item}}</td><td class="count">{{.Count}}</td></tr>
          {{end}}
        </tbody>
      </table>
    </div>
  </main>
</body>
</html>
`

func run(inputPath, outputPath string) error {
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inputPath, err)
	}

	var loot lootData
	if err := json.Unmarshal(input, &loot); err != nil {
		return fmt.Errorf("decode %s: %w", inputPath, err)
	}

	mobNames := make([]string, 0, len(loot.Mobs))
	for name := range loot.Mobs {
		mobNames = append(mobNames, name)
	}
	sort.Slice(mobNames, func(i, j int) bool {
		iBoss := isBoss(mobNames[i])
		jBoss := isBoss(mobNames[j])
		if iBoss != jBoss {
			return iBoss
		}
		return strings.ToLower(mobNames[i]) < strings.ToLower(mobNames[j])
	})

	data := pageData{MobCount: len(mobNames)}
	for group, mobName := range mobNames {
		items := loot.Mobs[mobName].Items
		itemNames := make([]string, 0, len(items))
		for itemName := range items {
			itemNames = append(itemNames, itemName)
		}
		sort.Slice(itemNames, func(i, j int) bool {
			if items[itemNames[i]] != items[itemNames[j]] {
				return items[itemNames[i]] > items[itemNames[j]]
			}
			return strings.ToLower(displayItemName(itemNames[i])) < strings.ToLower(displayItemName(itemNames[j]))
		})

		for index, itemName := range itemNames {
			count := items[itemName]
			data.Rows = append(data.Rows, row{
				Mob: mobName, Item: displayItemName(itemName), Count: count,
				First: index == 0, Span: len(itemNames), Group: group % 2,
			})
			data.ItemCount++
			data.TotalDrops += count
		}
	}

	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outputPath, err)
	}
	if err := template.Must(template.New("loot").Parse(page)).Execute(output, data); err != nil {
		output.Close()
		return fmt.Errorf("render %s: %w", outputPath, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close %s: %w", outputPath, err)
	}
	return nil
}

func main() {
	inputPath := "loot.json"
	outputPath := "loot.html"
	if len(os.Args) > 3 {
		fmt.Fprintln(os.Stderr, "usage: lootreport [loot.json] [loot.html]")
		os.Exit(2)
	}
	if len(os.Args) >= 2 {
		inputPath = os.Args[1]
	}
	if len(os.Args) == 3 {
		outputPath = os.Args[2]
	}
	if err := run(inputPath, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "lootreport: %v\n", err)
		os.Exit(1)
	}
}
