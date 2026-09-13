package packing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

var ErrForbidden = errors.New("only the owner can do that")
var ErrAccessRemoved = errors.New("access to this shared resource was removed")

const invitationTTLSeconds int32 = 7 * 24 * 60 * 60

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

// shareLink is a separate Cosmos item so the invitation capability can be
// removed by Cosmos TTL without expiring the packing list or session itself.
// Invitation state remains embedded in the resource document so approval and
// membership are still committed atomically.
type shareLink struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	Type       string    `json:"type"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resourceId"`
	Hash       string    `json:"hash"`
	Expires    time.Time `json:"expires"`
	TTL        int32     `json:"ttl"`
}

func (l InvitationLink) Hash() string {
	sum := sha256.Sum256([]byte(l.Token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func (l InvitationLink) Path() string {
	return "/join/" + l.Owner + "/" + l.Kind + "/" + l.ID + "/" + l.Token
}
func shareLinkID(hash string) string { return "share-link:" + hash }
func resourceKind(kind string) bool  { return kind == "packing-list" || kind == "packing-session" }
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
		if kind == "packing-session" {
			var trip PackingSession
			if err = json.Unmarshal(data, &trip); err != nil {
				return err
			}
			trip.expandPersonalEntries()
			if len(trip.List.Items)+len(trip.List.Tasks) > 2000 {
				return ErrInvalid
			}
			doc["list"], err = json.Marshal(trip.List)
			if err != nil {
				return err
			}
			data, err = json.Marshal(doc)
			if err != nil {
				return err
			}
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
	expires := s.clock().UTC().Add(time.Duration(invitationTTLSeconds) * time.Second)
	capability := shareLink{
		ID:         shareLinkID(link.Hash()),
		UserID:     actor,
		Type:       "share-link",
		Kind:       kind,
		ResourceID: id,
		Hash:       link.Hash(),
		Expires:    expires,
		TTL:        invitationTTLSeconds,
	}
	data, err := json.Marshal(capability)
	if err != nil {
		return link, err
	}
	if _, err = s.container.CreateItem(ctx, azcosmos.NewPartitionKeyString(actor), data, nil); err != nil {
		return link, err
	}
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
		sharing.Invitations = append(sharing.Invitations, Invitation{Hash: link.Hash(), Expires: expires, Status: "open"})
		return nil
	})
	if err != nil {
		// The capability grants nothing on its own. Remove it eagerly when the
		// authoritative invitation could not be added; TTL remains the fallback.
		_, _ = s.container.DeleteItem(ctx, azcosmos.NewPartitionKeyString(actor), capability.ID, nil)
	}
	return link, err
}

func (s *Store) readShareLink(ctx context.Context, link InvitationLink) error {
	response, err := s.container.ReadItem(ctx, azcosmos.NewPartitionKeyString(link.Owner), shareLinkID(link.Hash()), nil)
	if err != nil {
		if isMissing(err) {
			return ErrNotFound
		}
		return err
	}
	var capability shareLink
	if err = json.Unmarshal(response.Value, &capability); err != nil {
		return ErrNotFound
	}
	if capability.ID != shareLinkID(link.Hash()) || capability.UserID != link.Owner || capability.Type != "share-link" || capability.Kind != link.Kind || capability.ResourceID != link.ID || capability.Hash != link.Hash() || capability.TTL != invitationTTLSeconds || !capability.Expires.After(s.clock()) {
		return ErrNotFound
	}
	return nil
}

func (s *Store) InvitationStatus(ctx context.Context, link InvitationLink, actor string) (string, error) {
	if !link.valid() {
		return "", ErrNotFound
	}
	if err := s.readShareLink(ctx, link); err != nil {
		return "", err
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
		active := view.Sharing.Invitations[:0]
		for _, invitation := range view.Sharing.Invitations {
			if invitation.Expires.After(s.clock()) {
				active = append(active, invitation)
			}
		}
		view.Sharing.Invitations = active
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

// ListTrips returns the owner's current (unarchived) trips started from a list,
// newest first. These are the trips offered when approving a list request.
func (s *Store) ListTrips(ctx context.Context, listID, actor string) ([]PackingSession, error) {
	trips, err := s.ownedListTrips(ctx, listID, actor)
	if err != nil {
		return nil, err
	}
	active, _ := PartitionSessions(trips, s.clock())
	return active, nil
}

func (s *Store) ownedListTrips(ctx context.Context, listID, actor string) ([]PackingSession, error) {
	_, head, err := s.resource(ctx, "packing-list", listID, actor)
	if err != nil {
		return nil, err
	}
	if head.UserID != actor {
		return nil, ErrForbidden
	}
	query := "SELECT * FROM sessions s WHERE s.userId = @userID AND s.type = 'packing-session' AND (NOT IS_DEFINED(s.deletedAt) OR IS_NULL(s.deletedAt))"
	pager := s.container.NewQueryItemsPager(query, azcosmos.NewPartitionKeyString(actor), &azcosmos.QueryOptions{QueryParameters: []azcosmos.QueryParameter{{Name: "@userID", Value: actor}}})
	sessions, err := mapPackingSessions(ctx, pager)
	if err != nil {
		return nil, err
	}
	trips := []PackingSession{}
	for _, trip := range sessions {
		if trip.UserID == actor && trip.TemplateID() == listID {
			trips = append(trips, trip)
		}
	}
	return trips, nil
}

// ApproveInvitationWithTrips approves a list request and also adds the same
// approved account to the chosen trips started from that list. The owner's
// approval is the grant, so no separate trip invitation link is needed.
// The list approval commits first and trips are granted only to an account that
// is then a list member: a failed trip grant leaves the list shared, and
// resubmitting is idempotent. A removed member gets no trips from a stale approval.
func (s *Store) ApproveInvitationWithTrips(ctx context.Context, listID, actor, hash string, tripIDs []string) error {
	if len(tripIDs) == 0 {
		return s.DecideInvitation(ctx, "packing-list", listID, actor, hash, true)
	}
	trips, err := s.ownedListTrips(ctx, listID, actor)
	if err != nil {
		return err
	}
	owned := map[string]bool{}
	for _, trip := range trips {
		owned[trip.ID] = true
	}
	chosen := map[string]bool{}
	for _, id := range tripIDs {
		if !owned[id] {
			return ErrNotFound
		}
		chosen[id] = true
	}
	if err = s.DecideInvitation(ctx, "packing-list", listID, actor, hash, true); err != nil {
		return err
	}
	_, head, err := s.canonical(ctx, "packing-list", listID, actor)
	if err != nil {
		return err
	}
	var applicant *Member
	for _, inv := range head.Sharing.Invitations {
		if inv.Hash == hash && inv.Status == "approved" && inv.Applicant != nil {
			applicant = inv.Applicant
		}
	}
	if applicant == nil {
		return ErrInvalid
	}
	if _, ok := head.Sharing.Members[applicant.Subject]; !ok {
		return ErrInvalid
	}
	for id := range chosen {
		if err = s.addTripMember(ctx, id, actor, *applicant); err != nil {
			return err
		}
	}
	return nil
}

// addTripMember mirrors RequestAccess and DecideInvitation for an owner-approved
// account: the discovery reference first, then membership in one conditional write.
func (s *Store) addTripMember(ctx context.Context, tripID, owner string, member Member) error {
	if err := s.writeReference(ctx, "packing-session", tripID, owner, member.Subject); err != nil {
		return err
	}
	return s.updateSharing(ctx, "packing-session", tripID, owner, func(sharing *Sharing) error {
		if _, ok := sharing.Members[member.Subject]; ok {
			return nil
		}
		if len(sharing.Members) >= 20 {
			return fmt.Errorf("%w: sharing limit reached", ErrInvalid)
		}
		if sharing.Members == nil {
			sharing.Members = map[string]Member{}
		}
		sharing.Members[member.Subject] = member
		return nil
	})
}

// writeReference creates a member's discovery reference to a resource. An
// existing reference is fine: it grants nothing on its own.
func (s *Store) writeReference(ctx context.Context, kind, id, owner, subject string) error {
	ref := sharedReference{ID: referenceID(kind, id), UserID: subject, Type: "shared-reference", Kind: kind, ResourceID: id, Owner: owner}
	data, _ := json.Marshal(ref)
	if _, err := s.container.CreateItem(ctx, azcosmos.NewPartitionKeyString(subject), data, nil); err != nil {
		var response *azcore.ResponseError
		if !errors.As(err, &response) || response.StatusCode != 409 {
			return err
		}
	}
	return nil
}

// ErrNotListMember rejects a trip member who is not (or no longer) a member of
// the list the trip starts from.
var ErrNotListMember = fmt.Errorf("%w: not a member of this list", ErrInvalid)

// TripMemberChoices lists who actor may add to a trip started from this list,
// ordered by name. Only the owner manages list membership and sees its members,
// so only the owner is offered them; everyone else starts a private trip.
func (l PackingList) TripMemberChoices(actor string) []Member {
	if l.UserID != actor {
		return nil
	}
	members := make([]Member, 0, len(l.Sharing.Members))
	for _, member := range l.Sharing.Members {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].Name != members[j].Name {
			return members[i].Name < members[j].Name
		}
		return members[i].Subject < members[j].Subject
	})
	return members
}

func chosenTripMembers(list PackingList, actor string, subjects []string) (map[string]Member, error) {
	if len(subjects) == 0 {
		return nil, nil
	}
	if list.UserID != actor {
		return nil, ErrForbidden
	}
	chosen := map[string]Member{}
	for _, subject := range subjects {
		member, ok := list.Sharing.Members[subject]
		if !ok || subject == actor {
			return nil, ErrNotListMember
		}
		chosen[subject] = member
	}
	return chosen, nil
}
