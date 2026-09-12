package packing

import (
	"context"
	"encoding/json"
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
		keyBytes, _ := json.Marshal([]string{user, op.ID})
		key := string(keyBytes)
		previous, ok := session.Operations[key]
		if !ok && user == session.UserID {
			previous, ok = session.Operations[op.ID]
		}
		if ok {
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
		if item.Revision != op.ExpectedRevision && item.Checked != op.Checked {
			return false, ErrConflict
		}
		if session.Operations == nil {
			session.Operations = map[string]PackingOperation{}
		}
		if item.Revision == op.ExpectedRevision {
			item.Checked = op.Checked
			item.Revision++
			item.UpdatedAt = time.Now().UTC()
			item.ChangedBy = "Trip owner"
			if member, ok := session.Sharing.Members[user]; ok {
				item.ChangedBy = member.Name
				if item.ChangedBy == "" {
					item.ChangedBy = "Another camper"
				}
			}
		}
		session.Operations[key] = op
		return true, nil
	})
}
