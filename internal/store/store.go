package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cablewindow/internal/domain"
)

type requestRecord struct {
	Signature string                  `json:"signature"`
	CaseID    string                  `json:"case_id"`
	Revision  int64                   `json:"revision"`
	Case      *domain.MaintenanceCase `json:"case"`
}

type FileStore struct {
	dir         string
	auditPath   string
	requestPath string
	locks       keyedLocks
	global      sync.Mutex
	requests    map[string]requestRecord
	events      []domain.AuditEvent
	ready       bool
}

func Open(dir string) (*FileStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("数据目录不能为空")
	}
	if err := os.MkdirAll(filepath.Join(dir, "cases"), 0o750); err != nil {
		return nil, err
	}
	s := &FileStore{dir: dir, auditPath: filepath.Join(dir, "audit.jsonl"), requestPath: filepath.Join(dir, "requests.json"), requests: map[string]requestRecord{}}
	if err := s.recover(); err != nil {
		return nil, err
	}
	s.ready = true
	return s, nil
}

func (s *FileStore) Ready() bool { return s.ready }

func (s *FileStore) LookupRequest(ctx context.Context, requestID, signature string) (domain.CommitResult, bool, error) {
	select {
	case <-ctx.Done():
		return domain.CommitResult{}, false, ctx.Err()
	default:
	}
	s.global.Lock()
	defer s.global.Unlock()
	record, ok := s.requests[requestID]
	if !ok {
		return domain.CommitResult{}, false, nil
	}
	if record.Signature != signature {
		return domain.CommitResult{}, true, domain.NewError("idempotency_conflict", "request_id 已用于不同命令")
	}
	return domain.CommitResult{Case: cloneCase(record.Case), Idempotent: true}, true, nil
}

func (s *FileStore) recover() error {
	if err := s.recoverTransactions(); err != nil {
		return err
	}
	if err := readJSON(s.requestPath, &s.requests); err != nil && !os.IsNotExist(err) {
		return err
	}
	events, err := loadAudit(s.auditPath)
	if err != nil {
		return err
	}
	s.events = events
	lastRevision := map[string]int64{}
	for _, event := range events {
		if event.FromRevision != lastRevision[event.CaseID] {
			return fmt.Errorf("个案 %s 审计修订不连续", event.CaseID)
		}
		lastRevision[event.CaseID] = event.ToRevision
	}
	entries, err := os.ReadDir(filepath.Join(s.dir, "cases"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var c domain.MaintenanceCase
		if err := readJSON(filepath.Join(s.dir, "cases", entry.Name()), &c); err != nil {
			return err
		}
		if c.Revision != lastRevision[c.ID] {
			return fmt.Errorf("个案 %s 快照修订与审计不一致", c.ID)
		}
	}
	return nil
}

func validID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	for _, r := range id {
		if !(r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

func (s *FileStore) casePath(id string) string { return filepath.Join(s.dir, "cases", id+".json") }

func (s *FileStore) Create(ctx context.Context, m domain.Mutation) (domain.CommitResult, error) {
	if m.Case == nil || !validID(m.Case.ID) {
		return domain.CommitResult{}, domain.NewError("invalid_id", "个案标识无效")
	}
	lock := s.locks.get(m.Case.ID)
	lock.Lock()
	defer lock.Unlock()
	if _, err := os.Stat(s.casePath(m.Case.ID)); err == nil {
		return domain.CommitResult{}, domain.NewError("case_exists", "个案标识已存在")
	}
	return s.persist(ctx, m, 0)
}

func (s *FileStore) Commit(ctx context.Context, m domain.Mutation) (domain.CommitResult, error) {
	if m.Case == nil || !validID(m.Case.ID) {
		return domain.CommitResult{}, domain.NewError("invalid_id", "个案标识无效")
	}
	lock := s.locks.get(m.Case.ID)
	lock.Lock()
	defer lock.Unlock()
	current, err := s.Get(ctx, m.Case.ID)
	if err != nil {
		return domain.CommitResult{}, err
	}
	return s.persistWithCurrent(ctx, m, current)
}

func (s *FileStore) persistWithCurrent(ctx context.Context, m domain.Mutation, current *domain.MaintenanceCase) (domain.CommitResult, error) {
	s.global.Lock()
	if record, ok := s.requests[m.RequestID]; ok {
		s.global.Unlock()
		if record.Signature != m.Signature {
			return domain.CommitResult{}, domain.NewError("idempotency_conflict", "request_id 已用于不同命令")
		}
		return domain.CommitResult{Case: cloneCase(record.Case), Idempotent: true}, nil
	}
	s.global.Unlock()
	if current.Revision != m.ExpectedRevision {
		return domain.CommitResult{}, domain.Conflict(current.Revision)
	}
	return s.persist(ctx, m, current.Revision)
}

func (s *FileStore) persist(ctx context.Context, m domain.Mutation, from int64) (domain.CommitResult, error) {
	select {
	case <-ctx.Done():
		return domain.CommitResult{}, ctx.Err()
	default:
	}
	if m.RequestID == "" || m.Signature == "" {
		return domain.CommitResult{}, domain.NewError("request_required", "request_id 与命令签名不能为空")
	}
	s.global.Lock()
	defer s.global.Unlock()
	if record, ok := s.requests[m.RequestID]; ok {
		if record.Signature != m.Signature {
			return domain.CommitResult{}, domain.NewError("idempotency_conflict", "request_id 已用于不同命令")
		}
		return domain.CommitResult{Case: cloneCase(record.Case), Idempotent: true}, nil
	}
	payload := sha256.Sum256([]byte(m.Signature))
	previous := ""
	if len(s.events) > 0 {
		previous = s.events[len(s.events)-1].EventHash
	}
	event := domain.AuditEvent{Sequence: int64(len(s.events) + 1), CaseID: m.Case.ID, RequestID: m.RequestID, EventType: m.EventType, Actor: m.Actor, OccurredAt: m.Case.UpdatedAt, FromRevision: from, ToRevision: m.Case.Revision, PayloadDigest: hex.EncodeToString(payload[:]), PreviousHash: previous}
	var err error
	event.EventHash, err = eventHash(event)
	if err != nil {
		return domain.CommitResult{}, err
	}
	record := requestRecord{Signature: m.Signature, CaseID: m.Case.ID, Revision: m.Case.Revision, Case: cloneCase(m.Case)}
	tx := transaction{Case: cloneCase(m.Case), Event: event, RequestID: m.RequestID, Record: copyRecord(record)}
	txPath, err := s.writeTransaction(tx)
	if err != nil {
		return domain.CommitResult{}, err
	}
	select {
	case <-ctx.Done():
		return domain.CommitResult{}, ctx.Err()
	default:
	}
	if err := atomicJSON(s.casePath(m.Case.ID), m.Case); err != nil {
		return domain.CommitResult{}, err
	}
	select {
	case <-ctx.Done():
		return domain.CommitResult{}, ctx.Err()
	default:
	}
	if err := appendEvent(s.auditPath, event); err != nil {
		return domain.CommitResult{}, err
	}
	select {
	case <-ctx.Done():
		return domain.CommitResult{}, ctx.Err()
	default:
	}
	s.requests[m.RequestID] = record
	if err := atomicJSON(s.requestPath, s.requests); err != nil {
		return domain.CommitResult{}, err
	}
	s.events = append(s.events, event)
	if err := os.Remove(txPath); err != nil {
		return domain.CommitResult{}, err
	}
	return domain.CommitResult{Case: cloneCase(m.Case)}, nil
}

func (s *FileStore) Get(ctx context.Context, id string) (*domain.MaintenanceCase, error) {
	if !validID(id) {
		return nil, domain.NewError("invalid_id", "个案标识无效")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	var c domain.MaintenanceCase
	if err := readJSON(s.casePath(id), &c); os.IsNotExist(err) {
		return nil, domain.NewError("not_found", "未找到维护个案")
	} else if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *FileStore) List(ctx context.Context) ([]*domain.MaintenanceCase, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, "cases"))
	if err != nil {
		return nil, err
	}
	list := make([]*domain.MaintenanceCase, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		c, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].UpdatedAt.Equal(list[j].UpdatedAt) {
			return list[i].ID < list[j].ID
		}
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list, nil
}

func (s *FileStore) FindOverlaps(ctx context.Context, segment string, start, end time.Time, excludeID string) ([]domain.WindowConflict, error) {
	cases, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var result []domain.WindowConflict
	for _, c := range cases {
		if c.ID == excludeID || c.CableSegment != segment || c.State == domain.StateClosed {
			continue
		}
		os, oe := start.UTC(), end.UTC()
		if c.WindowStart.After(os) {
			os = c.WindowStart
		}
		if c.WindowEnd.Before(oe) {
			oe = c.WindowEnd
		}
		if os.Before(oe) {
			result = append(result, domain.WindowConflict{CaseID: c.ID, State: c.State, OverlapStart: os, OverlapEnd: oe})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CaseID < result[j].CaseID })
	return result, nil
}

func (s *FileStore) Audit(ctx context.Context, caseID string, offset, limit int) ([]domain.AuditEvent, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	s.global.Lock()
	defer s.global.Unlock()
	filtered := make([]domain.AuditEvent, 0)
	for _, e := range s.events {
		if e.CaseID == caseID {
			filtered = append(filtered, e)
		}
	}
	if offset >= len(filtered) {
		return []domain.AuditEvent{}, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]domain.AuditEvent(nil), filtered[offset:end]...), nil
}

func (s *FileStore) VerifyAudit(ctx context.Context, caseID string) (domain.AuditVerification, error) {
	c, err := s.Get(ctx, caseID)
	if err != nil {
		return domain.AuditVerification{}, err
	}
	s.global.Lock()
	defer s.global.Unlock()
	verification := domain.AuditVerification{CaseID: caseID, Valid: true, Revision: c.Revision}
	var priorRevision int64
	previousHash := ""
	for _, event := range s.events {
		want, hashErr := eventHash(event)
		if hashErr != nil || want != event.EventHash || event.PreviousHash != previousHash {
			verification.Valid = false
		}
		previousHash = event.EventHash
		if event.CaseID != caseID {
			continue
		}
		if verification.EventCount == 0 {
			verification.FirstHash = event.EventHash
		}
		if event.FromRevision != priorRevision || event.ToRevision <= event.FromRevision {
			verification.Valid = false
		}
		priorRevision = event.ToRevision
		verification.LastHash = event.EventHash
		verification.EventCount++
	}
	if priorRevision != c.Revision || verification.EventCount == 0 {
		verification.Valid = false
	}
	return verification, nil
}

func (s *FileStore) AuditSummary(ctx context.Context, caseID string) (domain.AuditSummary, error) {
	verification, err := s.VerifyAudit(ctx, caseID)
	if err != nil {
		return domain.AuditSummary{}, err
	}
	if !verification.Valid {
		return domain.AuditSummary{}, domain.NewError("audit_invalid", "审计链验证未通过")
	}
	s.global.Lock()
	defer s.global.Unlock()
	summary := domain.AuditSummary{CaseID: caseID, EventCount: verification.EventCount, FirstHash: verification.FirstHash, LastHash: verification.LastHash, FinalRevision: verification.Revision}
	actorSet := map[string]bool{}
	typeCounts := map[string]int{}
	for _, event := range s.events {
		if event.CaseID != caseID {
			continue
		}
		if summary.FirstOccurredAt.IsZero() {
			summary.FirstOccurredAt = event.OccurredAt
		}
		summary.LastOccurredAt = event.OccurredAt
		actorSet[event.Actor] = true
		typeCounts[event.EventType]++
	}
	for actor := range actorSet {
		summary.Actors = append(summary.Actors, actor)
	}
	sort.Strings(summary.Actors)
	types := make([]string, 0, len(typeCounts))
	for eventType := range typeCounts {
		types = append(types, eventType)
	}
	sort.Strings(types)
	for _, eventType := range types {
		summary.EventTypes = append(summary.EventTypes, domain.EventTypeCount{EventType: eventType, Count: typeCounts[eventType]})
	}
	canonical, err := json.Marshal(struct {
		CaseID        string                  `json:"case_id"`
		EventCount    int                     `json:"event_count"`
		LastHash      string                  `json:"last_hash"`
		FinalRevision int64                   `json:"final_revision"`
		Actors        []string                `json:"actors"`
		EventTypes    []domain.EventTypeCount `json:"event_types"`
	}{summary.CaseID, summary.EventCount, summary.LastHash, summary.FinalRevision, summary.Actors, summary.EventTypes})
	if err != nil {
		return domain.AuditSummary{}, err
	}
	digest := sha256.Sum256(canonical)
	summary.Digest = hex.EncodeToString(digest[:])
	return summary, nil
}

func cloneCase(c *domain.MaintenanceCase) *domain.MaintenanceCase {
	if c == nil {
		return nil
	}
	b, _ := json.Marshal(c)
	var result domain.MaintenanceCase
	_ = json.Unmarshal(b, &result)
	return &result
}
