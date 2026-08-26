package workflow

import (
	"context"
	"time"

	"cablewindow/internal/domain"
)

type CaseView struct {
	Case    *domain.MaintenanceCase `json:"case"`
	Actions domain.ActionProjection `json:"actions"`
}

func (s *Service) Get(ctx context.Context, id string) (CaseView, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return CaseView{}, err
	}
	if c.State == domain.StateClosed && c.Archive == nil {
		if audit, e := s.repo.AuditSummary(ctx, id); e == nil {
			_ = c.BuildArchive(audit)
		}
	}
	return CaseView{Case: c, Actions: c.Actions(s.clock().UTC())}, nil
}

func (s *Service) ClosurePrecheck(ctx context.Context, id string) (domain.ClosurePrecheck, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.ClosurePrecheck{}, err
	}
	return c.ClosurePrecheck(s.clock().UTC()), nil
}

func (s *Service) Archive(ctx context.Context, id string) (*domain.ArchivePackage, error) {
	s.archiveMu.Lock()
	if cached := s.archiveCache[id]; cached != nil {
		s.archiveMu.Unlock()
		return cached, nil
	}
	s.archiveMu.Unlock()

	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.State != domain.StateClosed {
		return nil, domain.NewError("archive_unavailable", "仅已关闭个案可查询归档包")
	}
	if c.Archive == nil {
		audit, e := s.repo.AuditSummary(ctx, id)
		if e != nil {
			return nil, e
		}
		if e = c.BuildArchive(audit); e != nil {
			return nil, e
		}
	}
	s.archiveMu.Lock()
	s.archiveCache[id] = c.Archive
	s.archiveMu.Unlock()
	return c.Archive, nil
}

func (s *Service) List(ctx context.Context) ([]CaseView, error) {
	cases, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]CaseView, 0, len(cases))
	now := s.clock().UTC()
	for _, c := range cases {
		views = append(views, CaseView{Case: c, Actions: c.Actions(now)})
	}
	return views, nil
}

func (s *Service) Audit(ctx context.Context, id string, offset, limit int) ([]domain.AuditEvent, error) {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.Audit(ctx, id, offset, limit)
}

func (s *Service) VerifyAudit(ctx context.Context, id string) (domain.AuditVerification, error) {
	return s.repo.VerifyAudit(ctx, id)
}

func (s *Service) AuditSummary(ctx context.Context, id string) (domain.AuditSummary, error) {
	return s.repo.AuditSummary(ctx, id)
}

func (s *Service) Ready() bool { return s.repo.Ready() }

func (s *Service) Now() time.Time { return s.clock().UTC() }
