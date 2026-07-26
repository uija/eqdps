package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/uija/eqdps/legacy/internal/catalog"
)

const (
	everQuestTimestamp = "Mon Jan 02 15:04:05 2006"
	logtestUsage       = `usage: logtest -log <logfile> (-t "text" | -s "spell name")`
)

func main() {
	if err := run(os.Args[1:], time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "logtest: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, now time.Time) error {
	flags := flag.NewFlagSet("logtest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	logPath := flags.String("log", "", "EverQuest logfile to append")
	var text, spell optionalString
	flags.Var(&text, "t", "plain text to append")
	flags.Var(&spell, "s", "spell whose fade message should be appended")
	if err := flags.Parse(args); err != nil {
		return errors.New(logtestUsage)
	}
	if flags.NArg() != 0 || *logPath == "" || text.set == spell.set {
		return errors.New(logtestUsage)
	}

	message := text.value
	if spell.set {
		spells, err := catalog.Load()
		if err != nil {
			return err
		}
		found := false
		for _, configuredSpell := range spells {
			if configuredSpell.Name == spell.value {
				message = configuredSpell.FadeMessage
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("spell %q is not in the embedded catalogue", spell.value)
		}
	}

	info, err := os.Stat(*logPath)
	if err != nil {
		return fmt.Errorf("inspect logfile %q: %w", *logPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("logfile %q is not a regular file", *logPath)
	}
	file, err := os.OpenFile(*logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open logfile %q: %w", *logPath, err)
	}
	_, writeErr := fmt.Fprintf(file, "[%s] %s\n", now.Format(everQuestTimestamp), message)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("append logfile: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close logfile: %w", closeErr)
	}
	return nil
}

type optionalString struct {
	value string
	set   bool
}

func (value *optionalString) String() string {
	return value.value
}

func (value *optionalString) Set(text string) error {
	value.value = text
	value.set = true
	return nil
}
