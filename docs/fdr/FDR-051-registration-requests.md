# FDR-051: Registration Requests

**Status:** Experimental
**Last reviewed:** 2026-10-19

## Overview

Registration requests let an applicant ask for an account without the server
creating it at once. The applicant enters a login, an email address, a
password, and an optional note. An administrator then approves or denies the
request. Approval creates the account with the password that the applicant
chose.

This policy suits a server where administrators must decide who joins, and
where no email delivery is configured.

## Behavior

- Operators choose the `approval` account-creation policy in server
  configuration. Under this policy, email code registration and external
  auto-provisioning are off. Sign-in to existing accounts is unchanged.
- The registration page asks for a login, an email address, a password with
  confirmation, and an optional note of up to 1000 characters. Submitting the
  form creates a pending request and shows a confirmation. It does not sign the
  applicant in.
- The login and password rules match account creation. A login or email that
  belongs to an account, or to another pending request, is refused.
- Administrators with the `user.invite` permission see pending requests,
  oldest first. Approving a request creates the account with the login as its
  display name. Denying a request removes it. Neither action sends email.
- The applicant's email address is not attached to the account. The account
  has no verified email, so password recovery by email is unavailable until the
  owner adds one.
- A pending request expires after 14 days. The server holds at most 200 pending
  requests. More submissions are refused until a request is decided or expires.

## Privacy and storage

- A pending request is runtime state in `RUNTIME_STATE`. It is not domain
  history, so it is not written to `EVT`. Approving or denying it deletes it, and
  an undecided request expires on its own.
- The password is stored only as a bcrypt hash. The applicant's email address and
  note are kept until the decision, like other pending registration state.
- The server does not log the login, email address, note, or password.

## Non-goals

- Guest access. Chatto has no unauthenticated access to rooms, so this policy
  does not add it.
- Email verification of the applicant. A later release may verify the address
  before the request reaches administrators.
- Notifications to administrators. Administrators check the list themselves.

## Related

- [FDR-036](FDR-036-invite-links.md) covers invite links, the other way to
  control who joins.
- [FDR-047](FDR-047-first-run-setup.md) covers the first owner account.
