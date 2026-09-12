package packing

import (
	"context"
	"time"
)

type PackingOperation struct {
	ID               string `json:"id"`
	ItemID           string `json:"itemId"`
	Checked          bool   `json:"checked"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

// SyncSessionItem records the acknowledgement with the change in one conditional write.
func (s *Store) SyncSessionItem(ctx context.Context, sessionID, user string, op PackingOperation) (PackingSession, error) {
	if op.ID == "" || len(op.ID) > 100 || op.ItemID == "" || op.ExpectedRevision < 0 {
		return PackingSession{}, ErrInvalid
	}
	return s.updateSession(ctx, sessionID, user, func(session *PackingSession) (bool, error) {
		if previous, ok := session.Operations[op.ID]; ok {
			if previous != op {
				return false, ErrConflict
			}
			return false, nil
		}
		index, err := getItemIndexById(session.List, op.ItemID)
		if err != nil {
			return false, ErrNotFound
		}
		item := &session.List.Items[index]
		if item.Revision != op.ExpectedRevision {
			return false, ErrConflict
		}
		if session.Operations == nil {
			session.Operations = map[string]PackingOperation{}
		}
		item.Checked = op.Checked
		item.Revision++
		item.UpdatedAt = time.Now().UTC()
		session.Operations[op.ID] = op
		return true, nil
	})
}
