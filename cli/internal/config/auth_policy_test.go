package config

import "testing"

func TestApprovalPolicyReplacesDirectRegistration(t *testing.T) {
	t.Parallel()

	approval := AuthConfig{AccountCreationPolicy: AccountCreationPolicyApproval}
	if approval.DirectRegistrationOrDefault() {
		t.Fatal("approval policy must turn off direct email registration")
	}
	if !approval.RegistrationRequestsEnabled() {
		t.Fatal("approval policy must accept registration requests")
	}

	open := AuthConfig{}
	if !open.DirectRegistrationOrDefault() || open.RegistrationRequestsEnabled() {
		t.Fatal("the default open policy must keep direct registration and refuse requests")
	}
}
