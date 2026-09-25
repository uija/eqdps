package events

import (
	"regexp"
	"strings"

	"github.com/uija/eqdps/internal/data"
)

func prepareEventPatterns(event *data.EventConfig) {
	compile := func(expression string, cached **regexp.Regexp) {
		if expression == "" {
			*cached = nil
			return
		}
		if *cached == nil || (*cached).String() != expression {
			*cached, _ = regexp.Compile(expression)
		}
	}
	if event.Type == data.EventTypeRegexp {
		compile(event.Expression, &event.RegExp)
	}
	if event.Type == data.EventTypeRegexp || event.Type == data.EventTypeSpell {
		compile(event.ExpressionOthers, &event.RegExpOthers)
	}
}

// matchesEvent combines the selected targets before notifying, so Both cannot
// produce two notifications for a single log message. Empty patterns are disabled.
func matchesEvent(event data.EventConfig, message string) bool {
	textMatch := func() bool {
		return event.Expression != "" &&
			((event.FullExpression && strings.EqualFold(event.Expression, message)) ||
				strings.Contains(message, event.Expression))
	}
	othersMatch := func() bool {
		return event.ExpressionOthers != "" && event.RegExpOthers != nil && event.RegExpOthers.MatchString(message)
	}
	switch event.Type {
	case data.EventTypeString:
		return textMatch()
	case data.EventTypeSpell:
		return textMatch() || othersMatch()
	case data.EventTypeRegexp:
		return (event.Expression != "" && event.RegExp != nil && event.RegExp.MatchString(message)) || othersMatch()
	}
	return false
}
