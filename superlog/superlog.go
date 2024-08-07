package superlog

import (
	"fmt"
	"io"
	"log/syslog"
	"os"
	"path/filepath"

	"github.com/aerth/mostly/journalwriter"
)

// New returns a non-nil io.Writer. if p is zero, uses INFO priority. if err is not nil, os.Stderr is returned with the error.
func New(p journalwriter.Priority, usesyslog bool, usejournald bool, remotesyslog string) (io.Writer, error) {
	switch {
	case usesyslog || remotesyslog != "":
		netw := ""
		if remotesyslog != "" {
			netw = "udp"
		}
		syslogw, err := syslog.Dial(netw, remotesyslog, syslog.LOG_DEBUG|syslog.LOG_DAEMON, filepath.Base(os.Args[0]))
		if syslogw == nil {
			return os.Stderr, err
		}
		return syslogw, err
	case usejournald:
		if !journalwriter.Enabled() {
			return os.Stderr, fmt.Errorf("journal not enabled")
		}
		return journalwriter.JournalWriter{Priority: p}, nil
	default:
		return os.Stderr, nil
	}
}
