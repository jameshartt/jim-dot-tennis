-- Update all fixture venue locations to "@ <club name>" based on home team's club
-- This replaces "TBD" venues with proper club-based locations

-- Step 1: Check current venue location status
SELECT 
    COUNT(*) as total_fixtures,
    COUNT(CASE WHEN venue_location = 'TBD' THEN 1 END) as tbd_venues,
    COUNT(CASE WHEN venue_location LIKE '@%' THEN 1 END) as club_venues,
    COUNT(DISTINCT venue_location) as unique_venues
FROM fixtures;

-- Step 2: Show sample of current venues and what they would become
SELECT DISTINCT
    venue_location as current_venue,
    COUNT(*) as fixture_count
FROM fixtures 
GROUP BY venue_location
ORDER BY fixture_count DESC;

-- Step 3: Preview the transformation (TEST ONLY)
SELECT 
    f.id,
    f.venue_location as current_venue,
    t.name as home_team,
    c.name as home_club,
    (SELECT d.name FROM divisions d WHERE d.id = f.division_id) as division
FROM fixtures f
JOIN teams t ON f.home_team_id = t.id
JOIN clubs c ON t.club_id = c.id
WHERE f.venue_location IS NOT NULL
ORDER BY f.scheduled_date
LIMIT 10;

-- Step 4: Update all venue locations (uncomment to execute)
BEGIN TRANSACTION;

UPDATE fixtures 
SET venue_location = (
    SELECT c.name 
    FROM teams t 
    JOIN clubs c ON t.club_id = c.id 
    WHERE t.id = fixtures.home_team_id
),
updated_at = CURRENT_TIMESTAMP
WHERE venue_location IS NOT NULL;

-- Verify the changes
SELECT 
    COUNT(*) as total_fixtures,
    COUNT(CASE WHEN venue_location LIKE '@%' THEN 1 END) as updated_venues,
    COUNT(CASE WHEN venue_location = 'TBD' THEN 1 END) as remaining_tbd
FROM fixtures;

-- Show sample of updated venues
SELECT DISTINCT
    venue_location,
    COUNT(*) as fixture_count
FROM fixtures 
GROUP BY venue_location
ORDER BY fixture_count DESC
LIMIT 10;

-- If everything looks correct, COMMIT; otherwise ROLLBACK
COMMIT; 