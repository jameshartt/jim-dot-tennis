-- WI-154: several tournament providers per installation, chosen per person.
--
-- A provider is a separate set of tournaments on the tournament server (for
-- example one for the club and one for a league cup).
--
-- courthive_provider_id is the tournament server's id for the provider. A row
-- with a non-empty id is one the app manages: it has taken the provider on
-- and keeps people's membership of it in line (internal/tournaments). A row
-- with '' is only a calendar to sync, as before.
--
-- The club's own provider was set up before this migration (WI-145): its id
-- is in app_settings and is copied onto its row here. Those settings stay and
-- still mean the club's own provider.
ALTER TABLE tournament_providers ADD COLUMN courthive_provider_id TEXT NOT NULL DEFAULT '';

UPDATE tournament_providers
SET courthive_provider_id = (SELECT value FROM app_settings WHERE key = 'tournament_provider_id')
WHERE EXISTS (SELECT 1 FROM app_settings WHERE key = 'tournament_provider_id' AND value <> '')
  AND UPPER(TRIM(provider_abbr)) =
      (SELECT UPPER(TRIM(value)) FROM app_settings WHERE key = 'tournament_provider_abbr');

-- The providers picked for a person. The rule that reads it is
-- tournaments.ProvidersFor: an administrator has every managed provider
-- whatever is here; anyone else with tournament access has the ones picked,
-- and with none picked the club's own provider only.
CREATE TABLE user_tournament_providers (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tournament_provider_id INTEGER NOT NULL REFERENCES tournament_providers(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, tournament_provider_id)
);
