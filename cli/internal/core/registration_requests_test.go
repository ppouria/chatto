package core

import (
	"errors"
	"strings"
	"testing"
)

func TestRegistrationRequestApprovalCreatesAccountWithSubmittedPassword(t *testing.T) {
	t.Parallel()

	c, _ := setupTestCore(t)
	ctx := testContext(t)
	adminID := invitationAdmin(t, c)

	submitted, err := c.SubmitRegistrationRequest(ctx, "applicant-one", "applicant-one@example.com", "password123", "I am the new hire")
	if err != nil {
		t.Fatalf("SubmitRegistrationRequest: %v", err)
	}

	pending, err := c.ListRegistrationRequests(ctx, adminID)
	if err != nil {
		t.Fatalf("ListRegistrationRequests: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != submitted.ID || pending[0].Email != "applicant-one@example.com" {
		t.Fatalf("pending = %+v, want the submitted request", pending)
	}

	approved, err := c.ApproveRegistrationRequest(ctx, adminID, submitted.ID)
	if err != nil {
		t.Fatalf("ApproveRegistrationRequest: %v", err)
	}
	if approved.Login != "applicant-one" {
		t.Fatalf("approved login = %q, want applicant-one", approved.Login)
	}
	if _, err := c.VerifyPassword(ctx, "applicant-one", "password123"); err != nil {
		t.Fatalf("approved account rejects the submitted password: %v", err)
	}

	pending, err = c.ListRegistrationRequests(ctx, adminID)
	if err != nil {
		t.Fatalf("ListRegistrationRequests after approval: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("approved request is still pending: %+v", pending)
	}
	if _, err := c.ApproveRegistrationRequest(ctx, adminID, submitted.ID); !errors.Is(err, ErrRegistrationRequestNotFound) {
		t.Fatalf("second approval error = %v, want ErrRegistrationRequestNotFound", err)
	}
}

func TestRegistrationRequestDenialCreatesNoAccount(t *testing.T) {
	t.Parallel()

	c, _ := setupTestCore(t)
	ctx := testContext(t)
	adminID := invitationAdmin(t, c)

	submitted, err := c.SubmitRegistrationRequest(ctx, "applicant-two", "applicant-two@example.com", "password123", "")
	if err != nil {
		t.Fatalf("SubmitRegistrationRequest: %v", err)
	}
	if err := c.DenyRegistrationRequest(ctx, adminID, submitted.ID); err != nil {
		t.Fatalf("DenyRegistrationRequest: %v", err)
	}
	if _, err := c.GetUserByLogin(ctx, "applicant-two"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("denied request created an account: %v", err)
	}
	if err := c.DenyRegistrationRequest(ctx, adminID, submitted.ID); !errors.Is(err, ErrRegistrationRequestNotFound) {
		t.Fatalf("second denial error = %v, want ErrRegistrationRequestNotFound", err)
	}
}

func TestRegistrationRequestStoresOnlyPasswordHash(t *testing.T) {
	t.Parallel()

	c, _ := setupTestCore(t)
	ctx := testContext(t)

	submitted, err := c.SubmitRegistrationRequest(ctx, "applicant-three", "applicant-three@example.com", "password123", "")
	if err != nil {
		t.Fatalf("SubmitRegistrationRequest: %v", err)
	}
	entry, err := c.storage.runtimeStateKV.Get(ctx, registrationRequestKey(submitted.ID))
	if err != nil {
		t.Fatalf("read stored request: %v", err)
	}
	if strings.Contains(string(entry.Value()), "password123") {
		t.Fatal("stored registration request contains the plaintext password")
	}
}

func TestRegistrationRequestRejectsDuplicatesAndInvalidInput(t *testing.T) {
	t.Parallel()

	c, _ := setupTestCore(t)
	ctx := testContext(t)

	if _, err := c.SubmitRegistrationRequest(ctx, "applicant-four", "applicant-four@example.com", "password123", ""); err != nil {
		t.Fatalf("SubmitRegistrationRequest: %v", err)
	}

	tests := []struct {
		name     string
		login    string
		email    string
		password string
		note     string
		want     error
	}{
		{name: "same login pending", login: "Applicant-Four", email: "other@example.com", password: "password123", want: ErrRegistrationRequestConflict},
		{name: "same email pending", login: "applicant-five", email: "applicant-four@example.com", password: "password123", want: ErrRegistrationRequestConflict},
		{name: "invalid email", login: "applicant-six", email: "not-an-email", password: "password123", want: ErrInvalidArgument},
		{name: "short password", login: "applicant-seven", email: "applicant-seven@example.com", password: "short", want: ErrPasswordTooShort},
		{name: "note too long", login: "applicant-eight", email: "applicant-eight@example.com", password: "password123", note: strings.Repeat("n", MaxRegistrationRequestNoteRunes+1), want: ErrRegistrationRequestNoteTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := c.SubmitRegistrationRequest(ctx, tt.login, tt.email, tt.password, tt.note); !errors.Is(err, tt.want) {
				t.Fatalf("SubmitRegistrationRequest error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRegistrationRequestDecisionsRequireUserInvite(t *testing.T) {
	t.Parallel()

	c, _ := setupTestCore(t)
	ctx := testContext(t)
	plain, err := c.CreateUser(ctx, SystemActorID, "plain-member", "Plain Member", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	submitted, err := c.SubmitRegistrationRequest(ctx, "applicant-nine", "applicant-nine@example.com", "password123", "")
	if err != nil {
		t.Fatalf("SubmitRegistrationRequest: %v", err)
	}
	if _, err := c.ListRegistrationRequests(ctx, plain.Id); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("list error = %v, want ErrPermissionDenied", err)
	}
	if _, err := c.ApproveRegistrationRequest(ctx, plain.Id, submitted.ID); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("approve error = %v, want ErrPermissionDenied", err)
	}
	if err := c.DenyRegistrationRequest(ctx, plain.Id, submitted.ID); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("deny error = %v, want ErrPermissionDenied", err)
	}
}
