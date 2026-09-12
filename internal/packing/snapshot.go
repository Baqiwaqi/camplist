package packing

import "time"

// SessionSnapshot is the deliberately limited packing view sent to a device.
// Memberships, applicants, invite hashes, receipts and reviews never cross this boundary.
type SessionSnapshot struct {
	Name      string            `json:"name"`
	ID        string            `json:"id"`
	UserID    string            `json:"userId"`
	AccountID string            `json:"accountId"`
	Shared    bool              `json:"shared"`
	CreatedAt time.Time         `json:"createdAt"`
	List      ChecklistSnapshot `json:"list"`
}
type ChecklistSnapshot struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Items []PackingItem `json:"items"`
}

func (s PackingSession) Snapshot(actor string) SessionSnapshot {
	items := append([]PackingItem{}, s.List.Items...)
	for _, task := range s.List.Tasks {
		items = append(items, PackingItem{ID: task.ID, Kind: "task", Name: task.Name, Checked: task.Done, Revision: task.Revision, Scope: task.Scope, Assignee: task.Assignee, AssigneeName: task.AssigneeName, ChangedBy: task.ChangedBy})
	}
	return SessionSnapshot{Name: s.DisplayName(), ID: s.ID, UserID: s.UserID, AccountID: actor, Shared: s.IsShared(), CreatedAt: s.CreatedAt, List: ChecklistSnapshot{ID: s.List.ID, Name: s.List.Name, Items: items}}
}
func (s PackingSession) IsShared() bool {
	return len(s.Sharing.Members) > 0 || len(s.Sharing.Invitations) > 0
}
func (l PackingList) IsShared() bool {
	return len(l.Sharing.Members) > 0 || len(l.Sharing.Invitations) > 0
}
