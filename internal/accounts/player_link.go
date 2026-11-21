// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// ActionPlayerLink: an administrator linked an account to a player record,
// changed the link, or removed it (users.player_id; "My teams" on the
// planning dashboard comes from it).
const ActionPlayerLink = "user.player_link"

// SetPlayerLink links account id to the player playerID, or removes the link
// when playerID is "". The player must exist (ErrPlayerNotFound) and must not
// be linked to another account (ErrPlayerLinked). It returns the account as
// it is afterwards.
func (s *Service) SetPlayerLink(ctx context.Context, actor Actor, id int64, playerID string) (Account, error) {
	playerID = strings.TrimSpace(playerID)
	var after Account
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		before, err := getAccount(ctx, tx, `u.id = ?`, id)
		if err != nil {
			return err
		}
		detail := "linked_player=none"
		if playerID == "" {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET player_id = NULL WHERE id = ?`, id); err != nil {
				return fmt.Errorf("unlink player: %w", err)
			}
		} else {
			var exists, linked int
			if err := tx.GetContext(ctx, &exists, `SELECT COUNT(*) FROM players WHERE id = ?`, playerID); err != nil {
				return fmt.Errorf("check player: %w", err)
			}
			if exists == 0 {
				return ErrPlayerNotFound
			}
			if err := tx.GetContext(ctx, &linked, `SELECT COUNT(*) FROM users WHERE player_id = ? AND id <> ?`, playerID, id); err != nil {
				return fmt.Errorf("check player link: %w", err)
			}
			if linked > 0 {
				return ErrPlayerLinked
			}
			if _, err := tx.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ?`, playerID, id); err != nil {
				return fmt.Errorf("link player: %w", err)
			}
			detail = "linked_player=yes"
		}
		if (before.PlayerID != nil && *before.PlayerID == playerID) || (before.PlayerID == nil && playerID == "") {
			detail += " (no change)"
		}
		if after, err = getAccount(ctx, tx, `u.id = ?`, id); err != nil {
			return err
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionPlayerLink,
			TargetUserID: id, TargetUsername: before.Username, Detail: detail})
	})
	return after, err
}
