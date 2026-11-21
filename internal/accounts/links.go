// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Invite and recovery links.
//
// There is no email. A link is made by an administrator (users page) or by
// the operator on the server (`jim-dot-tennis user create|reset`), shown
// once, and handed over however the club likes. Whoever opens it can give the
// account a sign-in method (a passkey, or a password) and is signed in.
//
// Rules, all enforced here:
//   - the token is 256 random bits; only its SHA-256 hash is stored, so the
//     database, the audit trail and the logs never hold a usable link;
//   - a link works once. It is consumed in the same transaction that sets the
//     sign-in method, by an UPDATE that only matches an unused, unrevoked
//     link, so two requests racing on one link cannot both win;
//   - it expires (InviteLinkLifetime / RecoveryLinkLifetime) and can be
//     revoked; issuing a link for a user revokes that user's earlier unused
//     links;
//   - issuing a link changes nothing about the account: its password,
//     passkeys and sessions stay as they are until the link is used;
//   - a disabled account cannot use a link;
//   - using a link ends every session of the account.

// LinkKind says what a link is for.
type LinkKind string

const (
	// LinkInvite sets up the first sign-in method of a new account.
	LinkInvite LinkKind = "invite"
	// LinkRecovery lets someone who is locked out set a new sign-in method.
	LinkRecovery LinkKind = "recovery"
)

// How long a link can be used after it is issued.
const (
	InviteLinkLifetime   = 48 * time.Hour
	RecoveryLinkLifetime = time.Hour
)

// linkTokenBytes is the size of a link's random token (256 bits).
const linkTokenBytes = 32

var (
	// ErrLinkInvalid is the one error for a link that cannot be used: no such
	// link, already used, revoked, expired, or for a disabled account. Callers
	// must not tell these apart to the person holding the link.
	ErrLinkInvalid = errors.New("that link is not valid: it is wrong, has expired, or has already been used or cancelled")
	// ErrInviteHasSignIn: an invite was asked for an account that can already
	// sign in; such an account gets a recovery link.
	ErrInviteHasSignIn = errors.New("this account can already sign in, so it cannot be invited again: send a recovery link instead")
	// ErrLinkNotPending: the link to revoke is already used, revoked or unknown.
	ErrLinkNotPending = errors.New("that link has already been used or cancelled")
)

// Link is an invite or recovery link as stored. It never carries the token.
type Link struct {
	ID         int64       `db:"id"`
	UserID     int64       `db:"user_id"`
	Username   string      `db:"username"`
	Role       models.Role `db:"role"`
	UserActive bool        `db:"user_active"`
	Kind       LinkKind    `db:"kind"`
	CreatedBy  string      `db:"created_by"`
	CreatedAt  time.Time   `db:"created_at"`
	ExpiresAt  time.Time   `db:"expires_at"`
	UsedAt     *time.Time  `db:"used_at"`
	RevokedAt  *time.Time  `db:"revoked_at"`
}

// Live reports whether the link could still be used at now (leaving aside
// whether its account is active).
func (l Link) Live(now time.Time) bool {
	return l.UsedAt == nil && l.RevokedAt == nil && l.ExpiresAt.After(now)
}

// IssuedLink is a link that has just been made. Token is the secret part of
// its address and exists only here: show it once and do not store or log it.
type IssuedLink struct {
	Link
	Token string
	// Superseded is how many earlier unused links of the same user this one
	// cancelled.
	Superseded int64
}

// Path is the link's address on this site, without the host.
func (l IssuedLink) Path() string { return LinkPath(l.Token) }

// LinkPath is the address (without the host) of the link with this token.
func LinkPath(token string) string { return auth.AccountLinkPathPrefix + token }

// newLinkToken returns a random token and the hash that is stored for it.
func newLinkToken() (token, hash string, err error) {
	raw := make([]byte, linkTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate link: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashLinkToken(token), nil
}

func hashLinkToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// wellFormedLinkToken reports whether token has the shape newLinkToken makes.
func wellFormedLinkToken(token string) bool {
	if len(token) != base64.RawURLEncoding.EncodedLen(linkTokenBytes) {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil
}

// LinkBinding is the value a passkey ceremony begun from a link is tied to
// (auth.Passkeys.BeginLinkRegistration). Only someone holding the link can
// produce it, it is different for every link, and it is neither the token nor
// the stored hash, so it can sit in the server's memory for the few minutes a
// ceremony lasts without being a usable link.
func LinkBinding(token string) string {
	sum := sha256.Sum256([]byte("account-link-passkey-ceremony:" + token))
	return "link:" + hex.EncodeToString(sum[:])
}

const linkColumns = `
	l.id, l.user_id, u.username, u.role, u.is_active AS user_active, l.kind,
	l.created_by, l.created_at, l.expires_at, l.used_at, l.revoked_at`

func getLink(ctx context.Context, db sqlx.QueryerContext, where string, arg any) (Link, error) {
	var l Link
	err := sqlx.GetContext(ctx, db, &l, `SELECT `+linkColumns+` FROM account_links l JOIN users u ON u.id = l.user_id WHERE `+where, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrLinkInvalid
	}
	if err != nil {
		return Link{}, fmt.Errorf("look up link: %w", err)
	}
	return l, nil
}

// Invite makes a new active account with no password and an invite link for
// it, in one transaction. playerID optionally links the account to a player
// record (nil or "" for none).
func (s *Service) Invite(ctx context.Context, actor Actor, username string, role models.Role, playerID *string) (Account, IssuedLink, error) {
	var (
		account Account
		issued  IssuedLink
	)
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		var err error
		if account, err = s.createAccount(ctx, tx, actor, username, role, playerID); err != nil {
			return err
		}
		issued, err = s.issueLink(ctx, tx, actor, account, LinkInvite)
		return err
	})
	if err != nil {
		return Account{}, IssuedLink{}, err
	}
	return account, issued, nil
}

// IssueLink makes a link of kind for account userID and cancels that
// account's earlier unused links. The account itself is not changed: it keeps
// its password, passkeys and sessions until the link is used.
//
// An invite is only for an account with no sign-in method yet
// (ErrInviteHasSignIn otherwise). A link may be issued for a disabled
// account, but cannot be used until the account is enabled.
func (s *Service) IssueLink(ctx context.Context, actor Actor, userID int64, kind LinkKind) (IssuedLink, error) {
	if kind != LinkInvite && kind != LinkRecovery {
		return IssuedLink{}, fmt.Errorf("unknown link kind %q", kind)
	}
	var issued IssuedLink
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		// A write first, so this transaction holds the write lock before it
		// reads the account.
		if _, err := tx.ExecContext(ctx, `UPDATE users SET id = id WHERE id = ?`, userID); err != nil {
			return fmt.Errorf("issue link: %w", err)
		}
		account, err := getAccount(ctx, tx, `u.id = ?`, userID)
		if err != nil {
			return err
		}
		if kind == LinkInvite && account.SignInMethods() > 0 {
			return ErrInviteHasSignIn
		}
		issued, err = s.issueLink(ctx, tx, actor, account, kind)
		return err
	})
	if err != nil {
		return IssuedLink{}, err
	}
	return issued, nil
}

func (s *Service) issueLink(ctx context.Context, tx *sqlx.Tx, actor Actor, account Account, kind LinkKind) (IssuedLink, error) {
	token, hash, err := newLinkToken()
	if err != nil {
		return IssuedLink{}, err
	}
	now := s.now()
	lifetime := RecoveryLinkLifetime
	if kind == LinkInvite {
		lifetime = InviteLinkLifetime
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE account_links SET revoked_at = ?
		WHERE user_id = ? AND used_at IS NULL AND revoked_at IS NULL`, now, account.ID)
	if err != nil {
		return IssuedLink{}, fmt.Errorf("cancel earlier links: %w", err)
	}
	superseded, _ := res.RowsAffected()
	res, err = tx.ExecContext(ctx, `
		INSERT INTO account_links (user_id, kind, token_hash, created_by, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, account.ID, kind, hash, actor.String(), now, now.Add(lifetime))
	if err != nil {
		return IssuedLink{}, fmt.Errorf("store link: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return IssuedLink{}, fmt.Errorf("store link: %w", err)
	}
	link, err := getLink(ctx, tx, `l.id = ?`, id)
	if err != nil {
		return IssuedLink{}, err
	}
	detail := fmt.Sprintf("kind=%s link_id=%d expires=%s superseded=%d", kind, id, link.ExpiresAt.UTC().Format(time.RFC3339), superseded)
	if err := recordAudit(ctx, tx, Record{Time: now, Actor: actor, Action: ActionLinkCreate,
		TargetUserID: account.ID, TargetUsername: account.Username, Detail: detail}); err != nil {
		return IssuedLink{}, err
	}
	return IssuedLink{Link: link, Token: token, Superseded: superseded}, nil
}

// RevokeLink cancels the unused link id (ErrLinkNotPending when it is already
// used, cancelled or unknown) and returns it.
func (s *Service) RevokeLink(ctx context.Context, actor Actor, id int64) (Link, error) {
	var link Link
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		now := s.now()
		res, err := tx.ExecContext(ctx, `
			UPDATE account_links SET revoked_at = ?
			WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL`, now, id)
		if err != nil {
			return fmt.Errorf("cancel link: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrLinkNotPending
		}
		if link, err = getLink(ctx, tx, `l.id = ?`, id); err != nil {
			return err
		}
		return recordAudit(ctx, tx, Record{Time: now, Actor: actor, Action: ActionLinkRevoke,
			TargetUserID: link.UserID, TargetUsername: link.Username, Detail: fmt.Sprintf("kind=%s link_id=%d", link.Kind, id)})
	})
	if err != nil {
		return Link{}, err
	}
	return link, nil
}

// PendingLinks lists the links that can still be used, newest first.
func (s *Service) PendingLinks(ctx context.Context) ([]Link, error) {
	var all []Link
	err := s.db.SelectContext(ctx, &all, `SELECT `+linkColumns+`
		FROM account_links l JOIN users u ON u.id = l.user_id
		WHERE l.used_at IS NULL AND l.revoked_at IS NULL
		ORDER BY l.created_at DESC, l.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	now := s.now()
	live := all[:0]
	for _, l := range all {
		if l.Live(now) {
			live = append(live, l)
		}
	}
	return live, nil
}

// LookupLink returns the link with this token if it can be used now. It
// changes nothing. Every reason a link cannot be used is ErrLinkInvalid.
func (s *Service) LookupLink(ctx context.Context, token string) (Link, error) {
	if !wellFormedLinkToken(token) {
		return Link{}, ErrLinkInvalid
	}
	link, err := getLink(ctx, s.db, `l.token_hash = ?`, hashLinkToken(token))
	if err != nil {
		return Link{}, err
	}
	if !link.Live(s.now()) || !link.UserActive {
		return Link{}, ErrLinkInvalid
	}
	return link, nil
}

// Redemption is what the person using a link chose: exactly one of Password
// and Passkey.
type Redemption struct {
	// Password is the password they typed (checked against the policy).
	Password string
	// Passkey is a passkey verified by auth.Passkeys.VerifyLinkRegistration
	// for this link's account, not yet stored.
	Passkey *models.WebAuthnCredential
	// RemoveOtherMethods (recovery links only) removes every sign-in method
	// the account had before, leaving only the new one: for when a device was
	// lost or the account may be in someone else's hands. Without it a new
	// passkey is added alongside what was there, and a new password replaces
	// the old password and leaves the passkeys.
	RemoveOtherMethods bool
	// PasskeysUnavailable: passkeys cannot be used where the link was
	// opened, so the administrator passkey requirement does not apply (see
	// admin_passkeys.go). Leave it false unless that is so.
	PasskeysUnavailable bool
}

// Redeemed says what using a link did.
type Redeemed struct {
	Link            Link
	Method          string // "passkey" or "password"
	SessionsEnded   int64
	PasskeysRemoved int64
	PasswordRemoved bool
}

// RedeemLink uses the link: it gives the account the chosen sign-in method,
// ends every session of the account and marks the link used, all in one
// transaction. The link is consumed by that transaction's first statement,
// an UPDATE that only matches an unused, unrevoked link, so of any number of
// concurrent calls with the same link exactly one can succeed; the others
// (and any later call) get ErrLinkInvalid and change nothing. A call that
// fails for another reason (for example a password that does not meet the
// policy) leaves the link unused.
//
// The caller signs the browser in afterwards (auth.Service.StartSession).
func (s *Service) RedeemLink(ctx context.Context, token string, how Redemption) (Redeemed, error) {
	if (how.Password == "") == (how.Passkey == nil) {
		return Redeemed{}, errors.New("redeem link: give a password or a passkey, not both")
	}
	link, err := s.LookupLink(ctx, token)
	if err != nil {
		return Redeemed{}, err
	}
	var hash string
	out := Redeemed{Method: "passkey"}
	if how.Passkey != nil {
		if how.Passkey.UserID != link.UserID {
			return Redeemed{}, errors.New("redeem link: the passkey was verified for another account")
		}
	} else {
		out.Method = "password"
		// Checked and hashed before the transaction, so the slow hash does
		// not hold the database's write lock.
		if err := auth.ValidatePassword(how.Password, link.Username); err != nil {
			return Redeemed{}, err
		}
		if hash, err = auth.HashPassword(how.Password); err != nil {
			return Redeemed{}, fmt.Errorf("hash password: %w", err)
		}
	}

	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		now := s.now()
		res, err := tx.ExecContext(ctx, `
			UPDATE account_links SET used_at = ?
			WHERE token_hash = ? AND used_at IS NULL AND revoked_at IS NULL`, now, hashLinkToken(token))
		if err != nil {
			return fmt.Errorf("use link: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return ErrLinkInvalid
		}
		// Read again inside the transaction: the account may have been
		// disabled since LookupLink.
		l, err := getLink(ctx, tx, `l.token_hash = ?`, hashLinkToken(token))
		if err != nil {
			return err
		}
		if !l.ExpiresAt.After(now) || !l.UserActive || l.UserID != link.UserID {
			return ErrLinkInvalid
		}
		out.Link = l
		if how.Passkey == nil && l.Role == models.RoleAdmin && !how.PasskeysUnavailable {
			// An administrator sets up a passkey, not a password, while the
			// club requires it (rolled back: the link stays unused).
			if required, err := repository.AdminPasskeysRequired(ctx, tx); err != nil {
				return err
			} else if required {
				return ErrAdminPasskeyRequired
			}
		}

		remove := how.RemoveOtherMethods && l.Kind == LinkRecovery
		if remove {
			res, err := tx.ExecContext(ctx, `DELETE FROM webauthn_credentials WHERE user_id = ?`, l.UserID)
			if err != nil {
				return fmt.Errorf("remove passkeys: %w", err)
			}
			out.PasskeysRemoved, _ = res.RowsAffected()
		}
		if how.Passkey != nil {
			if remove {
				res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = NULL WHERE id = ? AND password_hash IS NOT NULL`, l.UserID)
				if err != nil {
					return fmt.Errorf("remove password: %w", err)
				}
				n, _ := res.RowsAffected()
				out.PasswordRemoved = n > 0
			}
			var held int
			if err := tx.GetContext(ctx, &held, `SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, l.UserID); err != nil {
				return fmt.Errorf("count passkeys: %w", err)
			}
			if held >= auth.MaxPasskeysPerUser {
				return auth.ErrPasskeyLimit
			}
			if err := repository.InsertWebAuthnCredential(ctx, tx, how.Passkey); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, must_change_password = FALSE WHERE id = ?`, hash, l.UserID); err != nil {
				return fmt.Errorf("set password: %w", err)
			}
		}
		if out.SessionsEnded, err = endSessions(ctx, tx, l.UserID); err != nil {
			return err
		}
		// Any other link of this account dies with this one.
		if _, err := tx.ExecContext(ctx, `
			UPDATE account_links SET revoked_at = ?
			WHERE user_id = ? AND id <> ? AND used_at IS NULL AND revoked_at IS NULL`, now, l.UserID, l.ID); err != nil {
			return fmt.Errorf("cancel other links: %w", err)
		}
		var detail strings.Builder
		fmt.Fprintf(&detail, "kind=%s link_id=%d method=%s sessions_ended=%d", l.Kind, l.ID, out.Method, out.SessionsEnded)
		if l.Kind == LinkRecovery {
			fmt.Fprintf(&detail, " removed_other_methods=%v passkeys_removed=%d password_removed=%v", remove, out.PasskeysRemoved, out.PasswordRemoved)
		}
		return recordAudit(ctx, tx, Record{Time: now, Actor: UserActor(l.UserID, l.Username), Action: ActionLinkUse,
			TargetUserID: l.UserID, TargetUsername: l.Username, Detail: detail.String()})
	})
	if err != nil {
		return Redeemed{}, err
	}
	return out, nil
}
