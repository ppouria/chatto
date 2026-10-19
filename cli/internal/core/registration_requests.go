package core

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	runtimestatev1 "github.com/chattocorp/chatto/cli/internal/pb/chatto/core/runtime_state/v1"
)

const (
	// RegistrationRequestTTL is how long a pending request waits for an administrator.
	// The runtime-state record expires on its own after this period.
	RegistrationRequestTTL = 14 * 24 * time.Hour
	// MaxPendingRegistrationRequests caps the review queue, so anonymous submissions
	// cannot grow it without limit.
	MaxPendingRegistrationRequests = 200
	// MaxRegistrationRequestNoteRunes bounds the note an applicant sends with a request.
	MaxRegistrationRequestNoteRunes = 1000

	registrationRequestKeyPrefix = "registration_request."
)

var (
	ErrRegistrationRequestNotFound    = errors.New("registration request not found")
	ErrRegistrationRequestsFull       = errors.New("too many pending registration requests")
	ErrRegistrationRequestConflict    = errors.New("a pending registration request already uses this login or email")
	ErrRegistrationRequestEmailTaken  = errors.New("email address is already in use")
	ErrRegistrationRequestNoteTooLong = errors.New("registration request note is too long")
)

// RegistrationRequestView is the part of a pending request that administrators see. It omits the
// password hash, which is used only when the request is approved.
type RegistrationRequestView struct {
	ID        string
	Login     string
	Email     string
	Note      string
	CreatedAt time.Time
}

func registrationRequestView(state *runtimestatev1.RegistrationRequestState) RegistrationRequestView {
	return RegistrationRequestView{
		ID:        state.GetId(),
		Login:     state.GetLogin(),
		Email:     state.GetEmail(),
		Note:      state.GetNote(),
		CreatedAt: state.GetCreatedAt().AsTime(),
	}
}

func registrationRequestKey(id string) string {
	return registrationRequestKeyPrefix + id
}

// SubmitRegistrationRequest stores an applicant's account request for an administrator to review.
// It applies the login, password, and email rules that account creation applies, and it hashes the
// password before anything is stored. Callers must check the server's approval policy first.
func (c *ChattoCore) SubmitRegistrationRequest(ctx context.Context, login, email, password, note string) (RegistrationRequestView, error) {
	login = strings.TrimSpace(login)
	email = normalizeRegistrationEmail(email)
	note = strings.TrimSpace(note)

	if err := ValidateLogin(login); err != nil {
		return RegistrationRequestView{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return RegistrationRequestView{}, err
	}
	if parsed, err := mail.ParseAddress(email); err != nil || parsed.Address != email {
		return RegistrationRequestView{}, fmt.Errorf("invalid email address: %w", ErrInvalidArgument)
	}
	if utf8.RuneCountInString(note) > MaxRegistrationRequestNoteRunes {
		return RegistrationRequestView{}, ErrRegistrationRequestNoteTooLong
	}
	if c.configModel.IsUsernameBlocked(login) || c.loginConflictsWithMentionHandle(login) {
		return RegistrationRequestView{}, ErrUsernameBlocked
	}
	if _, err := c.GetUserByLogin(ctx, login); err == nil {
		return RegistrationRequestView{}, ErrLoginAlreadyTaken
	} else if !errors.Is(err, ErrNotFound) {
		return RegistrationRequestView{}, err
	}
	claimed, err := c.IsEmailClaimed(ctx, email)
	if err != nil {
		return RegistrationRequestView{}, err
	}
	if claimed {
		return RegistrationRequestView{}, ErrRegistrationRequestEmailTaken
	}

	pending, err := c.listRegistrationRequestStates(ctx)
	if err != nil {
		return RegistrationRequestView{}, err
	}
	if len(pending) >= MaxPendingRegistrationRequests {
		return RegistrationRequestView{}, ErrRegistrationRequestsFull
	}
	for _, state := range pending {
		if strings.EqualFold(state.GetLogin(), login) || state.GetEmail() == email {
			return RegistrationRequestView{}, ErrRegistrationRequestConflict
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordHashCost)
	if err != nil {
		return RegistrationRequestView{}, fmt.Errorf("failed to hash password: %w", err)
	}
	state := &runtimestatev1.RegistrationRequestState{
		Id:           newID("Q"),
		Login:        login,
		Email:        email,
		Note:         note,
		PasswordHash: hash,
		CreatedAt:    timestamppb.New(time.Now().UTC()),
	}
	data, err := proto.Marshal(state)
	if err != nil {
		return RegistrationRequestView{}, fmt.Errorf("failed to marshal registration request: %w", err)
	}
	if _, err := c.storage.runtimeStateKV.Create(ctx, registrationRequestKey(state.GetId()), data, jetstream.KeyTTL(RegistrationRequestTTL)); err != nil {
		return RegistrationRequestView{}, fmt.Errorf("failed to store registration request: %w", err)
	}
	return registrationRequestView(state), nil
}

// ListRegistrationRequests returns the pending requests, oldest first. It requires user.invite.
func (c *ChattoCore) ListRegistrationRequests(ctx context.Context, actorID string) ([]RegistrationRequestView, error) {
	if err := c.requireServerPermission(ctx, actorID, PermUserInvite); err != nil {
		return nil, err
	}
	states, err := c.listRegistrationRequestStates(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]RegistrationRequestView, 0, len(states))
	for _, state := range states {
		views = append(views, registrationRequestView(state))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].CreatedAt.Before(views[j].CreatedAt) })
	return views, nil
}

// ApproveRegistrationRequest creates the account for a pending request and then removes the request.
// The account gets the request's password hash. The login is also the display name. The applicant's
// email is not attached to the account. It requires user.invite.
func (c *ChattoCore) ApproveRegistrationRequest(ctx context.Context, actorID, id string) (RegistrationRequestView, error) {
	if err := c.requireServerPermission(ctx, actorID, PermUserInvite); err != nil {
		return RegistrationRequestView{}, err
	}
	entry, state, err := c.getRegistrationRequestEntry(ctx, id)
	if err != nil {
		return RegistrationRequestView{}, err
	}
	if _, err := c.createUserWithOptions(ctx, actorID, state.GetLogin(), state.GetLogin(), "", userCreationOptions{passwordHash: state.GetPasswordHash()}); err != nil {
		return RegistrationRequestView{}, err
	}
	if err := c.storage.runtimeStateKV.Delete(ctx, entry.Key(), jetstream.LastRevision(entry.Revision())); err != nil {
		// The account exists already. The request is left behind and expires on its own.
		c.logger.Warn("approved registration request was not removed", "request_id", id, "error", err)
	}
	return registrationRequestView(state), nil
}

// DenyRegistrationRequest removes a pending request without creating an account. It requires user.invite.
func (c *ChattoCore) DenyRegistrationRequest(ctx context.Context, actorID, id string) error {
	if err := c.requireServerPermission(ctx, actorID, PermUserInvite); err != nil {
		return err
	}
	entry, _, err := c.getRegistrationRequestEntry(ctx, id)
	if err != nil {
		return err
	}
	if err := c.storage.runtimeStateKV.Delete(ctx, entry.Key(), jetstream.LastRevision(entry.Revision())); err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return ErrRegistrationRequestNotFound
		}
		return fmt.Errorf("failed to delete registration request: %w", err)
	}
	return nil
}

func (c *ChattoCore) getRegistrationRequestEntry(ctx context.Context, id string) (jetstream.KeyValueEntry, *runtimestatev1.RegistrationRequestState, error) {
	if id == "" {
		return nil, nil, ErrRegistrationRequestNotFound
	}
	entry, err := c.storage.runtimeStateKV.Get(ctx, registrationRequestKey(id))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, nil, ErrRegistrationRequestNotFound
		}
		return nil, nil, fmt.Errorf("failed to get registration request: %w", err)
	}
	state := &runtimestatev1.RegistrationRequestState{}
	if err := proto.Unmarshal(entry.Value(), state); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal registration request: %w", err)
	}
	return entry, state, nil
}

func (c *ChattoCore) listRegistrationRequestStates(ctx context.Context) ([]*runtimestatev1.RegistrationRequestState, error) {
	lister, err := c.storage.runtimeStateKV.ListKeysFiltered(ctx, registrationRequestKeyPrefix+"*")
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list registration requests: %w", err)
	}
	var out []*runtimestatev1.RegistrationRequestState
	for key := range lister.Keys() {
		entry, err := c.storage.runtimeStateKV.Get(ctx, key)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				continue
			}
			return nil, fmt.Errorf("failed to get registration request: %w", err)
		}
		state := &runtimestatev1.RegistrationRequestState{}
		if err := proto.Unmarshal(entry.Value(), state); err != nil {
			return nil, fmt.Errorf("failed to unmarshal registration request: %w", err)
		}
		out = append(out, state)
	}
	return out, nil
}
