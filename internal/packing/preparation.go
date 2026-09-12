package packing

import (
	"context"
	"slices"
	"strings"
)

func (s *Store) EditPreparationTask(ctx context.Context, id, actor string, task PreparationTask, remove bool, revision string) error {
	list, err := s.GetPackingList(ctx, id, actor)
	if err != nil {
		return err
	}
	if revision == "" || revision != list.Revision() {
		return ErrConflict
	}
	if !validScope(task.Scope, false) {
		return ErrInvalid
	}
	task.Name = strings.TrimSpace(task.Name)
	if task.ID == "" || len(task.ID) > 100 || !remove && (task.Name == "" || len(task.Name) > 200) {
		return ErrInvalid
	}
	index := slices.IndexFunc(list.Tasks, func(current PreparationTask) bool { return current.ID == task.ID })
	if remove {
		if index < 0 {
			return ErrNotFound
		}
		list.Tasks = append(list.Tasks[:index], list.Tasks[index+1:]...)
	} else if index < 0 {
		if len(list.Tasks) >= 200 {
			return ErrInvalid
		}
		list.Tasks = append(list.Tasks, task)
	} else {
		list.Tasks[index] = task
	}
	return s.SavePackingList(ctx, list)
}

// SetSessionPreparationTask records preparation for this trip only.
func (s *Store) SetSessionPreparationTask(ctx context.Context, id, actor, taskID string, done bool, expectedRevision int64) (PackingSession, error) {
	if expectedRevision < 0 {
		return PackingSession{}, ErrInvalid
	}
	return s.updateSession(ctx, id, actor, func(session *PackingSession) (bool, error) {
		index := slices.IndexFunc(session.List.Tasks, func(task PreparationTask) bool { return task.ID == taskID })
		if index < 0 {
			return false, ErrNotFound
		}
		task := &session.List.Tasks[index]
		return task.setCompletion(actor, session.participantName(actor), done, expectedRevision)
	})
}

func (task *PreparationTask) setCompletion(actor, name string, done bool, revision int64) (bool, error) {
	if task.Assignee != "" && task.Assignee != actor {
		return false, ErrForbidden
	}
	if task.Done == done {
		return false, nil
	}
	if task.Revision != revision {
		return false, ErrConflict
	}
	task.Done = done
	task.Revision++
	task.ChangedBy = name
	return true, nil
}
