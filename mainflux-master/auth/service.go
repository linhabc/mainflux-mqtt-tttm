// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"time"

	"github.com/mainflux/mainflux"
	"github.com/mainflux/mainflux/internal/groups"
	"github.com/mainflux/mainflux/pkg/errors"
	"github.com/mainflux/mainflux/pkg/ulid"
)

const (
	loginDuration    = 10 * time.Hour
	recoveryDuration = 5 * time.Minute
)

var (
	// ErrUnauthorizedAccess represents unauthorized access.
	ErrUnauthorizedAccess = errors.New("unauthorized access")

	// ErrMalformedEntity indicates malformed entity specification (e.g.
	// invalid owner or ID).
	ErrMalformedEntity = errors.New("malformed entity specification")

	// ErrNotFound indicates a non-existing entity request.
	ErrNotFound = errors.New("entity not found")

	// ErrGenerateGroupID indicates error in creating group.
	ErrGenerateGroupID = errors.New("failed to generate group id")

	// ErrConflict indicates that entity already exists.
	ErrConflict = errors.New("entity already exists")

	// ErrFailedToRetrieveMembers failed to retrieve group members.
	ErrFailedToRetrieveMembers = errors.New("failed to retrieve group members")

	errIssueUser = errors.New("failed to issue new user key")
	errIssueTmp  = errors.New("failed to issue new temporary key")
	errRevoke    = errors.New("failed to remove key")
	errRetrieve  = errors.New("failed to retrieve key data")
	errIdentify  = errors.New("failed to validate token")
)

// Authn specifies an API that must be fullfiled by the domain service
// implementation, and all of its decorators (e.g. logging & metrics).
// Token is a string value of the actual Key and is used to authenticate
// an Auth service request.
type Authn interface {
	// Issue issues a new Key, returning its token value alongside.
	Issue(ctx context.Context, token string, key Key) (Key, string, error)

	// Revoke removes the Key with the provided id that is
	// issued by the user identified by the provided key.
	Revoke(ctx context.Context, token, id string) error

	// Retrieve retrieves data for the Key identified by the provided
	// ID, that is issued by the user identified by the provided key.
	RetrieveKey(ctx context.Context, token, id string) (Key, error)

	// Identify validates token token. If token is valid, content
	// is returned. If token is invalid, or invocation failed for some
	// other reason, non-nil error value is returned in response.
	Identify(ctx context.Context, token string) (Identity, error)
}

// Authz specifies an API for the authorization and will be implemented
// by evaluation of policies.
type Authz interface {
	// Authorize checks access rights
	Authorize(ctx context.Context, token, sub, obj, act string) (bool, error)
}

// Service specifies an API that must be fullfiled by the domain service
// implementation, and all of its decorators (e.g. logging & metrics).
// Token is a string value of the actual Key and is used to authenticate
// an Auth service request.
type Service interface {
	Authn
	Authz

	// Implements groups API, creating groups, assigning members
	groups.Service
}

var _ Service = (*service)(nil)

type service struct {
	keys         KeyRepository
	groups       groups.Repository
	uuidProvider mainflux.IDProvider
	ulidProvider mainflux.IDProvider
	tokenizer    Tokenizer
}

// New instantiates the auth service implementation.
func New(keys KeyRepository, groups groups.Repository, up mainflux.IDProvider, tokenizer Tokenizer) Service {
	return &service{
		tokenizer:    tokenizer,
		keys:         keys,
		groups:       groups,
		uuidProvider: up,
		ulidProvider: ulid.New(),
	}
}

func (svc service) Issue(ctx context.Context, token string, key Key) (Key, string, error) {
	if key.IssuedAt.IsZero() {
		return Key{}, "", ErrInvalidKeyIssuedAt
	}
	switch key.Type {
	case APIKey:
		return svc.userKey(ctx, token, key)
	case RecoveryKey:
		return svc.tmpKey(recoveryDuration, key)
	default:
		return svc.tmpKey(loginDuration, key)
	}
}

func (svc service) Revoke(ctx context.Context, token, id string) error {
	issuerID, _, err := svc.login(token)
	if err != nil {
		return errors.Wrap(errRevoke, err)
	}
	if err := svc.keys.Remove(ctx, issuerID, id); err != nil {
		return errors.Wrap(errRevoke, err)
	}
	return nil
}

func (svc service) RetrieveKey(ctx context.Context, token, id string) (Key, error) {
	issuerID, _, err := svc.login(token)
	if err != nil {
		return Key{}, errors.Wrap(errRetrieve, err)
	}

	return svc.keys.Retrieve(ctx, issuerID, id)
}

func (svc service) Identify(ctx context.Context, token string) (Identity, error) {
	key, err := svc.tokenizer.Parse(token)
	if err == ErrAPIKeyExpired {
		err = svc.keys.Remove(ctx, key.IssuerID, key.ID)
		return Identity{}, errors.Wrap(ErrAPIKeyExpired, err)
	}
	if err != nil {
		return Identity{}, errors.Wrap(errIdentify, err)
	}

	switch key.Type {
	case APIKey, RecoveryKey, UserKey:
		return Identity{ID: key.IssuerID, Email: key.Subject}, nil
	default:
		return Identity{}, ErrUnauthorizedAccess
	}
}

func (svc service) Authorize(ctx context.Context, token, sub, obj, act string) (bool, error) {
	return true, nil
}

func (svc service) tmpKey(duration time.Duration, key Key) (Key, string, error) {
	key.ExpiresAt = key.IssuedAt.Add(duration)
	secret, err := svc.tokenizer.Issue(key)
	if err != nil {
		return Key{}, "", errors.Wrap(errIssueTmp, err)
	}

	return key, secret, nil
}

func (svc service) userKey(ctx context.Context, token string, key Key) (Key, string, error) {
	id, sub, err := svc.login(token)
	if err != nil {
		return Key{}, "", errors.Wrap(errIssueUser, err)
	}

	key.IssuerID = id
	if key.Subject == "" {
		key.Subject = sub
	}

	keyID, err := svc.uuidProvider.ID()
	if err != nil {
		return Key{}, "", errors.Wrap(errIssueUser, err)
	}
	key.ID = keyID

	if _, err := svc.keys.Save(ctx, key); err != nil {
		return Key{}, "", errors.Wrap(errIssueUser, err)
	}

	secret, err := svc.tokenizer.Issue(key)
	if err != nil {
		return Key{}, "", errors.Wrap(errIssueUser, err)
	}

	return key, secret, nil
}

func (svc service) login(token string) (string, string, error) {
	key, err := svc.tokenizer.Parse(token)
	if err != nil {
		return "", "", err
	}
	// Only user key token is valid for login.
	if key.Type != UserKey || key.IssuerID == "" {
		return "", "", ErrUnauthorizedAccess
	}

	return key.IssuerID, key.Subject, nil
}

func (svc service) CreateGroup(ctx context.Context, token string, g groups.Group) (string, error) {
	user, err := svc.Identify(ctx, token)
	if err != nil {
		return "", errors.Wrap(ErrUnauthorizedAccess, err)
	}

	ulid, err := svc.ulidProvider.ID()
	if err != nil {
		return "", errors.Wrap(ErrGenerateGroupID, err)
	}

	g.ID = ulid
	g.OwnerID = user.ID
	if _, err := svc.groups.Save(ctx, g); err != nil {
		return "", err
	}

	return g.ID, nil
}

func (svc service) ListGroups(ctx context.Context, token string, level uint64, gm groups.Metadata) (groups.GroupPage, error) {
	if _, err := svc.Identify(ctx, token); err != nil {
		return groups.GroupPage{}, errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.RetrieveAll(ctx, level, gm)

}

func (svc service) ListParents(ctx context.Context, token string, childID string, level uint64, gm groups.Metadata) (groups.GroupPage, error) {
	if _, err := svc.Identify(ctx, token); err != nil {
		return groups.GroupPage{}, errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.RetrieveAllParents(ctx, childID, level, gm)
}

func (svc service) ListChildren(ctx context.Context, token string, parentID string, level uint64, gm groups.Metadata) (groups.GroupPage, error) {
	if _, err := svc.Identify(ctx, token); err != nil {
		return groups.GroupPage{}, errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.RetrieveAllChildren(ctx, parentID, level, gm)
}

func (svc service) ListMembers(ctx context.Context, token, groupID string, offset, limit uint64, gm groups.Metadata) (groups.MemberPage, error) {
	if _, err := svc.Identify(ctx, token); err != nil {
		return groups.MemberPage{}, errors.Wrap(ErrUnauthorizedAccess, err)
	}
	p, err := svc.groups.Members(ctx, groupID, offset, limit, gm)
	if err != nil {
		return groups.MemberPage{}, errors.Wrap(ErrFailedToRetrieveMembers, err)
	}
	mp := groups.MemberPage{
		PageMetadata: groups.PageMetadata{
			Total:  p.Total,
			Offset: p.Offset,
			Limit:  p.Limit,
		},
		Members: make([]groups.Member, 0),
	}
	mp.Members = append(mp.Members, p.Members)
	return mp, nil
}

func (svc service) RemoveGroup(ctx context.Context, token, id string) error {
	if _, err := svc.Identify(ctx, token); err != nil {
		return errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.Delete(ctx, id)
}

func (svc service) Unassign(ctx context.Context, token, memberID, groupID string) error {
	if _, err := svc.Identify(ctx, token); err != nil {
		return errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.Unassign(ctx, memberID, groupID)
}

func (svc service) UpdateGroup(ctx context.Context, token string, g groups.Group) (groups.Group, error) {
	if _, err := svc.Identify(ctx, token); err != nil {
		return groups.Group{}, errors.Wrap(ErrUnauthorizedAccess, err)
	}

	return svc.groups.Update(ctx, g)
}

func (svc service) ViewGroup(ctx context.Context, token, id string) (groups.Group, error) {
	if _, err := svc.Identify(ctx, token); err != nil {
		return groups.Group{}, errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.RetrieveByID(ctx, id)
}

func (svc service) Assign(ctx context.Context, token, memberID, groupID string) error {
	if _, err := svc.Identify(ctx, token); err != nil {
		return errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.Assign(ctx, memberID, groupID)
}

func (svc service) ListMemberships(ctx context.Context, token string, memberID string, offset, limit uint64, gm groups.Metadata) (groups.GroupPage, error) {
	if _, err := svc.Identify(ctx, token); err != nil {
		return groups.GroupPage{}, errors.Wrap(ErrUnauthorizedAccess, err)
	}
	return svc.groups.Memberships(ctx, memberID, offset, limit, gm)
}
