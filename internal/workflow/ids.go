package workflow

import (
	"encoding/hex"
)

func (s *Service) newCaseID() string {
	b := make([]byte, 12)
	_, _ = s.ids.Read(b)
	return "case-" + hex.EncodeToString(b)
}

func (s *Service) newEntityID(prefix string) string {
	b := make([]byte, 12)
	_, _ = s.ids.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
