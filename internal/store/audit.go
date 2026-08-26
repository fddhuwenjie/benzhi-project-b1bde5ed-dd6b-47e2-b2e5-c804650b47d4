package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"cablewindow/internal/domain"
)

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

func loadAudit(path string) ([]domain.AuditEvent, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []domain.AuditEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	previous := ""
	for scanner.Scan() {
		var event domain.AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("审计记录损坏: %w", err)
		}
		if event.Sequence != int64(len(events)+1) || event.PreviousHash != previous {
			return nil, fmt.Errorf("审计链不连续，序号 %d", event.Sequence)
		}
		want, err := eventHash(event)
		if err != nil || want != event.EventHash {
			return nil, fmt.Errorf("审计哈希校验失败，序号 %d", event.Sequence)
		}
		previous = event.EventHash
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
