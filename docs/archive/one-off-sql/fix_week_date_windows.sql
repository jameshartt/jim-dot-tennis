-- Realign week date windows to the actual fixture calendar.
--
-- Background: CreateSeason generates week start/end dates by evenly dividing
-- the season span across 18 weeks (~9.4 days each for a 14 Apr - 30 Sep
-- season), so the windows drift weeks away from the real weekly fixture
-- calendar. By July 2026 the drift was ~4 league weeks: the week-14 fixtures
-- (played 14-16 Jul) sat inside the week-10 date window (7-15 Jul).
--
-- This rewrites each week's window to the Monday-Sunday calendar week most of
-- its fixtures are actually scheduled in (modal Monday, so single rescheduled
-- outliers don't skew the window). Weeks with no fixtures are left untouched.
--
-- The play-down eligibility rule (BHPLTA Rule 16) no longer depends on these
-- windows (it uses fixtures.week_id), but date-based "current week" lookups
-- (match card import default week, points, selection overview) do.
--
-- Usage (adjust season_id to the target season first!):
--   sqlite3 tennis.db < scripts/fix_week_date_windows.sql
--
-- Take a backup before running against production.

BEGIN TRANSACTION;

UPDATE weeks SET
  start_date = (SELECT wk_monday FROM (
      SELECT datetime(date(f.scheduled_date, '+1 day', 'weekday 1', '-7 days')) AS wk_monday, COUNT(*) AS c
      FROM fixtures f WHERE f.week_id = weeks.id
      GROUP BY wk_monday ORDER BY c DESC, wk_monday ASC LIMIT 1)),
  end_date = (SELECT datetime(wk_monday, '+6 days', '+23 hours', '+59 minutes', '+59 seconds') FROM (
      SELECT date(f.scheduled_date, '+1 day', 'weekday 1', '-7 days') AS wk_monday, COUNT(*) AS c
      FROM fixtures f WHERE f.week_id = weeks.id
      GROUP BY wk_monday ORDER BY c DESC, wk_monday ASC LIMIT 1)),
  updated_at = CURRENT_TIMESTAMP
WHERE season_id = 3  -- 2026 season; verify with: SELECT id, name FROM seasons WHERE is_active = 1;
  AND EXISTS (SELECT 1 FROM fixtures f WHERE f.week_id = weeks.id);

-- Review the result before committing elsewhere:
SELECT week_number, start_date, end_date FROM weeks WHERE season_id = 3 ORDER BY week_number;

COMMIT;
