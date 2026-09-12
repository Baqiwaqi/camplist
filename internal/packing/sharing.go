package packing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

var ErrForbidden = errors.New("only the owner can do that")
var ErrAccessRemoved = errors.New("access to this shared resource was removed")

type Member struct {
	Subject       string `json:"subject"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}
type Invitation struct {
	Hash      string    `json:"hash"`
	Expires   time.Time `json:"expires"`
	Status    string    `json:"status"`
	Applicant *Member   `json:"applicant,omitempty"`
}
type Sharing struct {
	Members     map[string]Member `json:"members,omitempty"`
	Invitations []Invitation      `json:"invitations,omitempty"`
}
type InvitationLink struct{ Kind, ID, Owner, Token string }

func (l InvitationLink) Hash() string {
	sum := sha256.Sum256([]byte(l.Token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func (l InvitationLink) Path() string {
	return "/join/" + l.Owner + "/" + l.Kind + "/" + l.ID + "/" + l.Token
}
func resourceKind(kind string) bool { return kind == "packing-list" || kind == "packing-session" }
func (l InvitationLink) valid() bool {
	b, err := base64.RawURLEncoding.DecodeString(l.Token)
	return resourceKind(l.Kind) && l.ID != "" && len(l.ID) <= 100 && l.Owner != "" && len(l.Owner) <= 200 && err == nil && len(b) == 32
}

type resourceHeader struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Type      string     `json:"type"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
	Sharing   Sharing    `json:"sharing,omitempty"`
}
type sharedReference struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	Type       string `json:"type"`
	Kind       string `json:"kind"`
	ResourceID string `json:"resourceId"`
	Owner      string `json:"owner"`
}

func referenceID(kind, id string) string { return "shared:" + kind + ":" + id }
func (s *Store) canonical(ctx context.Context, kind, id, owner string) (azcosmos.ItemResponse, resourceHeader, error) {
	response, err := s.container.ReadItem(ctx, azcosmos.NewPartitionKeyString(owner), id, nil)
	if err != nil {
		return response, resourceHeader{}, err
	}
	var head resourceHeader
	if err = json.Unmarshal(response.Value, &head); err != nil {
		return response, head, err
	}
	if head.Type != kind || head.ID != id || head.UserID != owner || head.DeletedAt != nil {
		return response, head, ErrNotFound
	}
	return response, head, nil
}

// Discovery locates a resource; its canonical document alone grants access.
func (s *Store) resource(ctx context.Context, kind, id, actor string) (azcosmos.ItemResponse, resourceHeader, error) {
	if !resourceKind(kind) {
		return azcosmos.ItemResponse{}, resourceHeader{}, ErrNotFound
	}
	response, head, err := s.canonical(ctx, kind, id, actor)
	if err == nil {
		return response, head, nil
	}
	if !isMissing(err) {
		return response, head, err
	}
	refResponse, refErr := s.container.ReadItem(ctx, azcosmos.NewPartitionKeyString(actor), referenceID(kind, id), nil)
	if refErr != nil {
		if isMissing(refErr) {
			return response, head, ErrNotFound
		}
		return response, head, refErr
	}
	var ref sharedReference
	if json.Unmarshal(refResponse.Value, &ref) != nil || ref.UserID != actor || ref.Kind != kind || ref.ResourceID != id || ref.Type != "shared-reference" {
		return response, head, ErrNotFound
	}
	response, head, err = s.canonical(ctx, kind, id, ref.Owner)
	if err != nil {
		return response, head, err
	}
	if _, ok := head.Sharing.Members[actor]; !ok {
		return response, head, ErrAccessRemoved
	}
	return response, head, nil
}
func (s *Store) updateSharing(ctx context.Context, kind, id, owner string, change func(*Sharing) error) error {
	for attempt := 0; attempt < 5; attempt++ {
		res, head, err := s.canonical(ctx, kind, id, owner)
		if err != nil {
			return err
		}
		if err = change(&head.Sharing); err != nil {
			return err
		}
		var doc map[string]json.RawMessage
		if err = json.Unmarshal(res.Value, &doc); err != nil {
			return err
		}
		doc["sharing"], err = json.Marshal(head.Sharing)
		if err != nil {
			return err
		}
		data, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		etag := res.ETag
		_, err = s.container.ReplaceItem(ctx, azcosmos.NewPartitionKeyString(owner), id, data, &azcosmos.ItemOptions{IfMatchEtag: &etag})
		if preconditionFailed(err) {
			continue
		}
		return err
	}
	return ErrConflict
}
func (s *Store) CreateInvitation(ctx context.Context, kind, id, actor string) (InvitationLink, error) {
	link := InvitationLink{Kind: kind, ID: id, Owner: actor}
	if !resourceKind(kind) {
		return link, ErrInvalid
	}
	_, head, err := s.resource(ctx, kind, id, actor)
	if err != nil {
		return link, err
	}
	if head.UserID != actor {
		return link, ErrForbidden
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return link, err
	}
	link.Token = base64.RawURLEncoding.EncodeToString(secret)
	err = s.updateSharing(ctx, kind, id, actor, func(sharing *Sharing) error {
		// Expired links cannot grant access and may be pruned to keep documents bounded.
		active := sharing.Invitations[:0]
		for _, inv := range sharing.Invitations {
			if inv.Expires.After(s.clock()) {
				active = append(active, inv)
			}
		}
		sharing.Invitations = active
		if len(sharing.Invitations) >= 20 || len(sharing.Members) >= 20 {
			return fmt.Errorf("%w: sharing limit reached", ErrInvalid)
		}
		sharing.Invitations = append(sharing.Invitations, Invitation{Hash: link.Hash(), Expires: s.clock().UTC().Add(7 * 24 * time.Hour), Status: "open"})
		return nil
	})
	return link, err
}
func (s *Store) InvitationStatus(ctx context.Context, link InvitationLink, actor string) (string, error) {
	if !link.valid() {
		return "", ErrNotFound
	}
	_, head, err := s.canonical(ctx, link.Kind, link.ID, link.Owner)
	if err != nil {
		return "", err
	}
	for _, inv := range head.Sharing.Invitations {
		if inv.Hash == link.Hash() && inv.Expires.After(s.clock()) && inv.Status != "revoked" && inv.Status != "rejected" {
			if inv.Applicant != nil && inv.Applicant.Subject != actor {
				return "", ErrNotFound
			}
			return inv.Status, nil
		}
	}
	return "", ErrNotFound
}
func (s *Store) RequestAccess(ctx context.Context, link InvitationLink, member Member) error {
	if member.Subject == "" || len(member.Subject) > 200 || len(member.Name) > 200 || len(member.Email) > 320 {
		return ErrInvalid
	}
	if _, err := s.InvitationStatus(ctx, link, member.Subject); err != nil {
		return err
	}
	if member.Subject == link.Owner {
		return ErrInvalid
	}
	// Write the repairable discovery index first. Pending/stale indexes grant no access.
	ref := sharedReference{ID: referenceID(link.Kind, link.ID), UserID: member.Subject, Type: "shared-reference", Kind: link.Kind, ResourceID: link.ID, Owner: link.Owner}
	data, _ := json.Marshal(ref)
	_, err := s.container.CreateItem(ctx, azcosmos.NewPartitionKeyString(member.Subject), data, nil)
	if err != nil {
		var response *azcore.ResponseError
		if !errors.As(err, &response) || response.StatusCode != 409 {
			return err
		}
	}
	return s.updateSharing(ctx, link.Kind, link.ID, link.Owner, func(sharing *Sharing) error {
		for i := range sharing.Invitations {
			inv := &sharing.Invitations[i]
			if inv.Hash != link.Hash() {
				continue
			}
			if !inv.Expires.After(s.clock()) || inv.Status == "revoked" || inv.Status == "rejected" {
				return ErrNotFound
			}
			if inv.Applicant != nil {
				if inv.Applicant.Subject == member.Subject {
					return nil
				}
				return ErrConflict
			}
			inv.Applicant = &member
			inv.Status = "pending"
			return nil
		}
		return ErrNotFound
	})
}
func (s *Store) DecideInvitation(ctx context.Context, kind, id, actor, hash string, approve bool) error {
	_, head, err := s.resource(ctx, kind, id, actor)
	if err != nil {
		return err
	}
	if head.UserID != actor {
		return ErrForbidden
	}
	return s.updateSharing(ctx, kind, id, actor, func(sharing *Sharing) error {
		for i := range sharing.Invitations {
			inv := &sharing.Invitations[i]
			if inv.Hash != hash {
				continue
			}
			if approve && inv.Status == "approved" {
				return nil
			}
			if !approve {
				inv.Status = "revoked"
				return nil
			}
			if inv.Status != "pending" || inv.Applicant == nil || !inv.Expires.After(s.clock()) {
				return ErrInvalid
			}
			if len(sharing.Members) >= 20 {
				return ErrInvalid
			}
			if sharing.Members == nil {
				sharing.Members = map[string]Member{}
			}
			sharing.Members[inv.Applicant.Subject] = *inv.Applicant
			inv.Status = "approved"
			return nil
		}
		return ErrNotFound
	})
}
func (s *Store) RemoveMember(ctx context.Context, kind, id, actor, subject string) error {
	_, head, err := s.resource(ctx, kind, id, actor)
	if err != nil {
		return err
	}
	if subject == head.UserID || actor != head.UserID && actor != subject {
		return ErrForbidden
	}
	return s.updateSharing(ctx, kind, id, head.UserID, func(sharing *Sharing) error {
		// A removed member cannot race another privileged operation.
		if actor != head.UserID {
			if _, ok := sharing.Members[actor]; !ok {
				return ErrAccessRemoved
			}
		}
		delete(sharing.Members, subject)
		return nil
	})
}

type SharingView struct {
	Kind, ID, Owner, Actor string
	Sharing                Sharing
}

func (s *Store) GetSharing(ctx context.Context, kind, id, actor string) (SharingView, error) {
	_, head, err := s.resource(ctx, kind, id, actor)
	if err != nil {
		return SharingView{}, err
	}
	view := SharingView{Kind: kind, ID: id, Owner: head.UserID, Actor: actor}
	if head.UserID == actor {
		view.Sharing = head.Sharing
	}
	return view, nil
}
func (s *Store) references(ctx context.Context, actor, kind string) ([]sharedReference, error) {
	pager := s.container.NewQueryItemsPager("SELECT * FROM c WHERE c.userId = @userID AND c.type = 'shared-reference'", azcosmos.NewPartitionKeyString(actor), &azcosmos.QueryOptions{QueryParameters: []azcosmos.QueryParameter{{Name: "@userID", Value: actor}}})
	refs := []sharedReference{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, data := range page.Items {
			var ref sharedReference
			if err = json.Unmarshal(data, &ref); err != nil {
				return nil, err
			}
			if ref.UserID == actor && ref.Kind == kind {
				refs = append(refs, ref)
			}
		}
	}
	return refs, nil
}
