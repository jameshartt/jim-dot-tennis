-- E2E Test Seed Data
-- Idempotent: uses INSERT OR REPLACE / INSERT OR IGNORE throughout
-- Run against the SQLite test database before E2E tests

-- ============================================================
-- Admin user: testadmin / testpassword123
-- ============================================================
INSERT OR REPLACE INTO users (id, username, password_hash, role, is_active, created_at, last_login_at)
VALUES (
  1,
  'testadmin',
  '$2b$12$l1qlpyQB/eSK2rr5RY3FV.fn1WJVWQmBh1G.0s/WC6TnrQOVj0DRK',
  'admin',
  1,
  '2025-01-01 00:00:00',
  '2025-01-01 00:00:00'
);

-- ============================================================
-- Season
-- ============================================================
INSERT OR REPLACE INTO seasons (id, name, year, start_date, end_date, is_active)
VALUES (1, 'Summer 2025', 2025, '2025-04-14', '2025-08-18', 1);

-- ============================================================
-- Weeks (18 weeks, Monday start dates)
-- ============================================================
INSERT OR REPLACE INTO weeks (id, week_number, season_id, start_date, end_date, name, is_active)
VALUES
  (1,  1,  1, '2025-04-14', '2025-04-20', 'Week 1',  1),
  (2,  2,  1, '2025-04-21', '2025-04-27', 'Week 2',  0),
  (3,  3,  1, '2025-04-28', '2025-05-04', 'Week 3',  0),
  (4,  4,  1, '2025-05-05', '2025-05-11', 'Week 4',  0),
  (5,  5,  1, '2025-05-12', '2025-05-18', 'Week 5',  0),
  (6,  6,  1, '2025-05-19', '2025-05-25', 'Week 6',  0),
  (7,  7,  1, '2025-05-26', '2025-06-01', 'Week 7',  0),
  (8,  8,  1, '2025-06-02', '2025-06-08', 'Week 8',  0),
  (9,  9,  1, '2025-06-09', '2025-06-15', 'Week 9',  0),
  (10, 10, 1, '2025-06-16', '2025-06-22', 'Week 10', 0),
  (11, 11, 1, '2025-06-23', '2025-06-29', 'Week 11', 0),
  (12, 12, 1, '2025-06-30', '2025-07-06', 'Week 12', 0),
  (13, 13, 1, '2025-07-07', '2025-07-13', 'Week 13', 0),
  (14, 14, 1, '2025-07-14', '2025-07-20', 'Week 14', 0),
  (15, 15, 1, '2025-07-21', '2025-07-27', 'Week 15', 0),
  (16, 16, 1, '2025-07-28', '2025-08-03', 'Week 16', 0),
  (17, 17, 1, '2025-08-04', '2025-08-10', 'Week 17', 0),
  (18, 18, 1, '2025-08-11', '2025-08-17', 'Week 18', 0);

-- ============================================================
-- League
-- ============================================================
INSERT OR REPLACE INTO leagues (id, name, type, year, region)
VALUES (1, 'Brighton & Hove Parks League', 'Parks', 2025, 'Brighton & Hove');

INSERT OR IGNORE INTO league_seasons (league_id, season_id) VALUES (1, 1);

-- ============================================================
-- Divisions
-- ============================================================
INSERT OR REPLACE INTO divisions (id, name, level, play_day, league_id, season_id, max_teams_per_club)
VALUES
  (1, 'Division 1', 1, 'Monday',  1, 1, 2),
  (2, 'Division 2', 2, 'Tuesday', 1, 1, 2);

-- ============================================================
-- Clubs (include website and timestamps to avoid NULL scan errors)
-- ============================================================
INSERT OR REPLACE INTO clubs (id, name, address, postcode, latitude, longitude, website, phone_number, created_at, updated_at)
VALUES
  (1, 'St Ann''s Tennis Club', '10 Egremont Place, Brighton', 'BN2 0GA', 50.8284, -0.1225, '', '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (2, 'Hove Park Tennis Club', 'Hove Park, Old Shoreham Rd, Hove', 'BN3 6HP', 50.8413, -0.1847, '', '', '2025-01-01 00:00:00', '2025-01-01 00:00:00');

-- ============================================================
-- Teams
-- ============================================================
INSERT OR REPLACE INTO teams (id, name, club_id, division_id, season_id, active)
VALUES
  (1, 'St Ann''s A',  1, 1, 1, 1),
  (2, 'St Ann''s B',  1, 2, 1, 1),
  (3, 'Hove Park A',  2, 1, 1, 1),
  (4, 'Hove Park B',  2, 2, 1, 1);

-- ============================================================
-- Players (8 players: 4 per club, mixed gender)
-- Clear fantasy_match_id from any existing player first to avoid unique constraint
-- ============================================================
UPDATE players SET fantasy_match_id = NULL WHERE fantasy_match_id = 1;

INSERT OR REPLACE INTO players (id, first_name, last_name, club_id, gender, fantasy_match_id)
VALUES
  ('p-alice',   'Alice',   'Smith',    1, 'Women', 1),
  ('p-bob',     'Bob',     'Johnson',  1, 'Men',   NULL),
  ('p-carol',   'Carol',   'Williams', 1, 'Women', NULL),
  ('p-dave',    'Dave',    'Brown',    1, 'Men',   NULL),
  ('p-eve',     'Eve',     'Davis',    2, 'Women', NULL),
  ('p-frank',   'Frank',   'Wilson',   2, 'Men',   NULL),
  ('p-grace',   'Grace',   'Taylor',   2, 'Women', NULL),
  ('p-henry',   'Henry',   'Anderson', 2, 'Men',   NULL);

-- ============================================================
-- Player-Team assignments
-- ============================================================
INSERT OR IGNORE INTO player_teams (player_id, team_id, season_id, is_active)
VALUES
  ('p-alice', 1, 1, 1), ('p-bob',   1, 1, 1),
  ('p-carol', 2, 1, 1), ('p-dave',  2, 1, 1),
  ('p-eve',   3, 1, 1), ('p-frank', 3, 1, 1),
  ('p-grace', 4, 1, 1), ('p-henry', 4, 1, 1);

-- ============================================================
-- Fixtures (1 per division in Week 1)
-- ============================================================
INSERT OR REPLACE INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status, notes, created_at, updated_at)
VALUES
  (1, 1, 3, 1, 1, 1, '2025-04-14 18:00:00', 'St Ann''s Tennis Club', 'Scheduled', '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (2, 2, 4, 2, 1, 1, '2025-04-15 18:00:00', 'St Ann''s Tennis Club', 'Scheduled', '', '2025-01-01 00:00:00', '2025-01-01 00:00:00');

-- ============================================================
-- Matchups for fixture 1 (Div 1: St Ann's A vs Hove Park A)
-- ============================================================
INSERT OR REPLACE INTO matchups (id, fixture_id, type, status, home_score, away_score, notes, created_at, updated_at)
VALUES
  (1, 1, 'Mens',      'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (2, 1, 'Womens',    'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (3, 1, '1st Mixed', 'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (4, 1, '2nd Mixed', 'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00');

-- ============================================================
-- Matchups for fixture 2 (Div 2: St Ann's B vs Hove Park B)
-- Used for destructive match result entry tests
-- ============================================================
INSERT OR REPLACE INTO matchups (id, fixture_id, type, status, home_score, away_score, notes, created_at, updated_at)
VALUES
  (5, 2, 'Mens',      'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (6, 2, 'Womens',    'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (7, 2, '1st Mixed', 'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
  (8, 2, '2nd Mixed', 'Pending', 0, 0, '', '2025-01-01 00:00:00', '2025-01-01 00:00:00');

-- ============================================================
-- Player availability (all Week 1 players available)
-- ============================================================
INSERT OR IGNORE INTO player_fixture_availability (player_id, fixture_id, status)
VALUES
  ('p-alice', 1, 'Available'), ('p-bob',   1, 'Available'),
  ('p-eve',   1, 'Available'), ('p-frank', 1, 'Available'),
  ('p-carol', 2, 'Available'), ('p-dave',  2, 'Available'),
  ('p-grace', 2, 'Available'), ('p-henry', 2, 'Unavailable');

-- ============================================================
-- Tennis players for fantasy mixed doubles (token-based auth)
-- ============================================================
INSERT OR REPLACE INTO tennis_players (id, first_name, last_name, common_name, nationality, gender, current_rank, highest_rank, year_pro, wikipedia_url, hand, birth_date, birth_place, tour)
VALUES
  (1, 'Aryna',     'Sabalenka', 'Aryna Sabalenka',  'BLR', 'Female', 1,  1, 2015, 'https://en.wikipedia.org/wiki/Aryna_Sabalenka',  'Right', '1998-05-05', 'Minsk',   'WTA'),
  (2, 'Novak',     'Djokovic',  'Novak Djokovic',   'SRB', 'Male',   1,  1, 2003, 'https://en.wikipedia.org/wiki/Novak_Djokovic',   'Right', '1987-05-22', 'Belgrade', 'ATP'),
  (3, 'Coco',      'Gauff',     'Coco Gauff',       'USA', 'Female', 3,  2, 2019, 'https://en.wikipedia.org/wiki/Coco_Gauff',       'Right', '2004-03-13', 'Atlanta',  'WTA'),
  (4, 'Jannik',    'Sinner',    'Jannik Sinner',    'ITA', 'Male',   2,  1, 2018, 'https://en.wikipedia.org/wiki/Jannik_Sinner',    'Right', '2001-08-16', 'Innichen','ATP');

INSERT OR REPLACE INTO fantasy_mixed_doubles (id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active)
VALUES (1, 1, 2, 3, 4, 'Sabalenka_Djokovic_Gauff_Sinner', 1);

-- p-alice is linked to the fantasy match via fantasy_match_id = 1 in the INSERT above
