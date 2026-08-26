package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"cablewindow/internal/domain"
)

type transaction struct {
	Case      *domain.MaintenanceCase `json:"case"`
	Event     domain.AuditEvent       `json:"event"`
	RequestID string                  `json:"request_id"`
	Record    requestRecord           `json:"record"`
}

func (s *FileStore) transactionPath(event domain.AuditEvent) string {
	return filepath.Join(s.dir, "transactions", event.EventHash+".json")
}

func (s *FileStore) writeTransaction(tx transaction) (string, error) {
	path := s.transactionPath(tx.Event)
	if err := atomicJSON(path, tx); err != nil {
		return "", err
	}
	return path, nil
}

func (s *FileStore) recoverTransactions() error {
	dir := filepath.Join(s.dir, "transactions")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var tx transaction
		if err := readJSON(path, &tx); err != nil {
			return err
		}
		if tx.Case == nil || tx.Event.EventHash == "" || tx.RequestID == "" {
			return domain.NewError("recovery_failed", "事务日志字段不完整")
		}
		events, err := loadAudit(s.auditPath)
		if err != nil {
			return err
		}
		found := false
		for _, event := range events {
			if event.EventHash == tx.Event.EventHash {
				found = true
				break
			}
		}
		if !found {
			previous := ""
			if len(events) > 0 {
				previous = events[len(events)-1].EventHash
			}
			if tx.Event.PreviousHash != previous || tx.Event.Sequence != int64(len(events)+1) {
				return domain.NewError("recovery_failed", "待恢复事务与审计链不连续")
			}
			if err := appendEvent(s.auditPath, tx.Event); err != nil {
				return err
			}
		}
		if err := atomicJSON(s.casePath(tx.Case.ID), tx.Case); err != nil {
			return err
		}
		requests := map[string]requestRecord{}
		if err := readJSON(s.requestPath, &requests); err != nil && !os.IsNotExist(err) {
			return err
		}
		requests[tx.RequestID] = tx.Record
		if err := atomicJSON(s.requestPath, requests); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyRecord(record requestRecord) requestRecord {
	b, _ := json.Marshal(record)
	var result requestRecord
	_ = json.Unmarshal(b, &result)
	return result
}
