package packing

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"
)

type PackingOperation struct {
	SaveForFuture    bool   `json:"saveForFuture,omitempty"`
	Action           string `json:"action,omitempty"`
	Kind             string `json:"kind,omitempty"`
	Name             string `json:"name,omitempty"`
	Category         string `json:"category,omitempty"`
	Scope            string `json:"scope,omitempty"`
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
	if op.SaveForFuture && op.Action != "add" {
		return PackingSession{}, ErrInvalid
	}
	if op.Action != "" && op.Action != "add" || op.Kind != "" && op.Kind != "task" || !validScope(op.Scope, true) {
		return PackingSession{}, ErrInvalid
	}
	session, err := s.updateSession(ctx, sessionID, user, func(session *PackingSession) (bool, error) {
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
		if op.Action == "add" {
			if strings.TrimSpace(op.Name) == "" || len(op.Name) > 200 || len(op.Category) > 100 || len(op.ItemID) > 100 {
				return false, ErrInvalid
			}
			for _, item := range session.List.Items {
				if item.ID == op.ItemID {
					return false, ErrConflict
				}
			}
			for _, task := range session.List.Tasks {
				if task.ID == op.ItemID {
					return false, ErrConflict
				}
			}
			if len(session.List.Items)+len(session.List.Tasks) >= 2000 {
				return false, ErrInvalid
			}
			assignee, name := "", ""
			if op.Scope == "mine" || op.Scope == "person" {
				assignee = user
				name = session.participantName(user)
			}
			if op.Kind == "task" {
				session.List.Tasks = append(session.List.Tasks, PreparationTask{ID: op.ItemID, Name: strings.TrimSpace(op.Name), Scope: op.Scope, SourceID: op.ItemID, Assignee: assignee, AssigneeName: name})
			} else {
				item := NewItem(strings.TrimSpace(op.Name), strings.TrimSpace(op.Category))
				item.ID = op.ItemID
				item.Scope = op.Scope
				item.SourceID = op.ItemID
				item.Assignee = assignee
				item.AssigneeName = name
				session.List.Items = append(session.List.Items, item)
			}
			session.expandPersonalEntries()
			if len(session.List.Items)+len(session.List.Tasks) > 2000 {
				return false, ErrInvalid
			}
			session.recordOperation(key, op)
			return true, nil
		}
		if op.Kind == "task" {
			index := slices.IndexFunc(session.List.Tasks, func(task PreparationTask) bool { return task.ID == op.ItemID })
			if index < 0 {
				return false, ErrNotFound
			}
			task := &session.List.Tasks[index]
			if task.Assignee != "" && task.Assignee != user {
				return false, ErrForbidden
			}
			if task.Revision != op.ExpectedRevision && task.Done != op.Checked {
				return false, ErrConflict
			}
			if task.Revision == op.ExpectedRevision {
				task.Done = op.Checked
				task.Revision++
				task.ChangedBy = session.participantName(user)
			}
			session.recordOperation(key, op)
			return true, nil
		}
		index, err := getItemIndexById(session.List, op.ItemID)
		if err != nil {
			return false, ErrNotFound
		}
		item := &session.List.Items[index]
		if item.Assignee != "" && item.Assignee != user {
			return false, ErrForbidden
		}
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
	if err == nil && op.SaveForFuture {
		err = s.rememberTripEntry(ctx, session, user, op)
	}
	return session, err
}
