package store

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"cablewindow/internal/domain"
)

// ErrAuditTailCorrupt indicates that the audit log ends with a final line that
// could not be decoded while all preceding lines form a valid hash chain. Such
// a tail may be repairable when a matching transaction can re-append the
// missing event.
var ErrAuditTailCorrupt = errors.New("审计尾部不完整")

func eventHash(e domain.AuditEvent) (string, error) {
	e.EventHash = ""
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func appendEvent(path string, event domain.AuditEvent) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if err = json.NewEncoder(f).Encode(event); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// auditTail describes an incomplete trailing record in the audit log. Offset is
// the byte position where the incomplete record begins, so callers may truncate
// the file there and re-append the correct event.
type auditTail struct {
	offset int64
}

// loadAudit reads and validates the full audit log. It returns the validated
// events and fails on any corruption, including an incomplete trailing line.
func loadAudit(path string) ([]domain.AuditEvent, error) {
	events, _, err := loadAuditWithTail(path)
	return events, err
}

// loadAuditWithTail reads the audit log and validates the prefix hash chain.
// When the final line cannot be decoded while every preceding line forms a
// valid, continuous hash chain, the returned events are the valid prefix, the
// returned error wraps ErrAuditTailCorrupt and tail points at the byte offset
// where the incomplete trailing record begins, so callers may truncate it.
// Any other corruption (a decode failure before the final line, a broken
// sequence/previous-hash/hash chain, or a non-final bad line) returns a
// descriptive error with a nil tail.
func loadAuditWithTail(path string) ([]domain.AuditEvent, *auditTail, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	var events []domain.AuditEvent
	previous := ""
	reader := bufio.NewReader(f)
	offset := int64(0)
	for {
		line, readErr := reader.ReadBytes('\n')
		start := offset
		offset += int64(len(line))
		hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
		if len(line) == 0 {
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return nil, nil, readErr
			}
			break
		}
		data := line
		if hasNewline {
			data = line[:len(line)-1]
		}
		if len(bytes.TrimSpace(data)) == 0 {
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return nil, nil, readErr
			}
			continue
		}
		var event domain.AuditEvent
		if decErr := json.Unmarshal(data, &event); decErr != nil {
			// Only a trailing line without a terminating newline is a candidate
			// for repair; the reached EOF means the write was interrupted. A
			// newline-terminated but undecodable line is mid-file corruption.
			if !hasNewline && errors.Is(readErr, io.EOF) {
				return events, &auditTail{offset: start}, fmt.Errorf("%w: %v", ErrAuditTailCorrupt, decErr)
			}
			return nil, nil, fmt.Errorf("审计记录损坏: %w", decErr)
		}
		if event.Sequence != int64(len(events)+1) || event.PreviousHash != previous {
			return nil, nil, fmt.Errorf("审计链不连续，序号 %d", event.Sequence)
		}
		want, hashErr := eventHash(event)
		if hashErr != nil || want != event.EventHash {
			return nil, nil, fmt.Errorf("审计哈希校验失败，序号 %d", event.Sequence)
		}
		previous = event.EventHash
		events = append(events, event)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, nil, readErr
		}
	}
	return events, nil, nil
}
