package journalwriter_test

import (
	"log"
	"testing"

	"github.com/aerth/mostly/journalwriter"
)

func TestJournal(t *testing.T) {
	log.SetOutput(journalwriter.GetJournalOrStderr(journalwriter.PriInfo))
	log.SetFlags(0) // time included already
	log.Printf("hello!")
	log.Printf("hello!")
	log.Printf("hello!")
	err := journalwriter.Send("everything is working", journalwriter.PriWarning, map[string]string{
		"SYSLOG_IDENTIFIER": "mostly", // instead of program_invocation_short_name(3)
		// cool link
		"DOCUMENTATION": "https://github.com/aerth/mostly",
	})
	println("check journalctl -f --user")
	if err != nil {
		t.Fatal(err)
	}
	Sender("Hi again")
}

// Sender example
func Sender(msg string) {
	_ = journalwriter.Send(msg, journalwriter.PriInfo, map[string]string{
		"SYSLOG_IDENTIFIER": "mostly", // instead of program_invocation_short_name(3)
	})
}
