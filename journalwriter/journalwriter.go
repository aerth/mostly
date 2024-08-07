package journalwriter

import (
	"io"
	"os"

	"github.com/coreos/go-systemd/journal"
)

type Priority = journal.Priority

var _ io.Writer = (*JournalWriter)(nil) // compile-time interface check

// JournalWriter writes to the systemd journal. If journal is not available, it falls back to FallbackWriter.
// It's an io.Writer, and log.SetOutput() can be used to set it as the default logger.
//
// var Info = JournalWriter{journal.PriInfo}
// var Err = JournalWriter{journal.PriErr}
// var Debug = JournalWriter{journal.PriDebug}
// var Warning = JournalWriter{journal.PriWarning}
// var Emergency = JournalWriter{journal.PriEmerg}
// var Alert = JournalWriter{journal.PriAlert}
// var Critical = JournalWriter{journal.PriCrit}
// var Notice = JournalWriter{journal.PriNotice}
type JournalWriter struct {
	Priority // default 0 is 'Emergency' level
}

// FallbackWriter is used when writing to journal fails
//
// If nil, write fails will be silent.
var FallbackWriter = os.Stderr

// DontLogErrors disables printing errors to FallbackWriter
var DontLogErrors = false

// DontFallback disables printing failed logs to FallbackWriter (set FallbackWriter nil to disable completely)
var DontFallback = false

// Write writes to the journal, falling back to stderr if journal is not available.
//
// See DontLogErrors and DontFallback to change behavior when errors occur.
func (j JournalWriter) Write(b []byte) (int, error) {
	err := journal.Send(string(b), j.Priority, nil)
	if err != nil {
		if FallbackWriter != nil {
			if !DontLogErrors {
				FallbackWriter.Write([]byte("journalwriter error: " + err.Error() + "\n"))
			}
			FallbackWriter.Write(b) // fallback to stderr
		}
		return 0, err
	}
	return len(b), nil
}

// GetJournalOrStderr checks if journal is enabled.
//
// if not, returns os.Stderr
//
// If p is zero, uses INFO level
func GetJournalOrStderr(p Priority) io.Writer {
	if p == 0 {
		p = journal.PriInfo
	}
	if !journal.Enabled() {
		return os.Stderr
	}
	return JournalWriter{p}
}

// Enabled checks whether the local systemd journal is available for logging.
func Enabled() bool {
	return journal.Enabled()
}
