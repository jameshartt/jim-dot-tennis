-- ROBUST UPDATE: Update all fixture times to 6pm BST (18:00:00+01:00)
-- This version handles different timestamp formats more safely

-- Step 1: Check current fixture time formats
SELECT 
    COUNT(*) as total_fixtures,
    COUNT(CASE WHEN scheduled_date LIKE '%00:00:00+00:00%' THEN 1 END) as midnight_utc_fixtures,
    COUNT(CASE WHEN scheduled_date LIKE '%18:00:00+01:00%' THEN 1 END) as already_six_pm_bst,
    MIN(scheduled_date) as earliest_date,
    MAX(scheduled_date) as latest_date
FROM fixtures;

-- Step 2: Show sample of current formats
SELECT 
    id,
    scheduled_date,
    length(scheduled_date) as string_length,
    substr(scheduled_date, 1, 10) as date_part,
    substr(scheduled_date, 12) as time_timezone_part
FROM fixtures 
ORDER BY scheduled_date 
LIMIT 5;

-- Step 3: Preview the transformation (TEST ONLY)
SELECT 
    id,
    scheduled_date as original,
    -- For standard format: 2025-08-05 00:00:00+00:00
    CASE 
        WHEN scheduled_date LIKE '____-__-__ __:__:__+__:__' THEN
            substr(scheduled_date, 1, 11) || '18:00:00+01:00'
        ELSE
            'FORMAT_NOT_RECOGNIZED: ' || scheduled_date
    END as new_scheduled_date,
    (SELECT name FROM divisions WHERE id = division_id) as division
FROM fixtures 
WHERE scheduled_date IS NOT NULL
ORDER BY scheduled_date 
LIMIT 10;

-- Step 4: If preview looks good, run the actual UPDATE
-- Uncomment the following section to execute:

BEGIN TRANSACTION;

UPDATE fixtures 
SET scheduled_date = CASE 
        WHEN scheduled_date LIKE '____-__-__ __:__:__+__:__' THEN
            substr(scheduled_date, 1, 11) || '18:00:00+01:00'
        ELSE
            scheduled_date -- Leave unchanged if format not recognized
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE scheduled_date IS NOT NULL
  AND scheduled_date LIKE '____-__-__ __:__:__+__:__';  -- Only update recognized formats

-- Verify the changes
SELECT 
    COUNT(*) as total_fixtures,
    COUNT(CASE WHEN scheduled_date LIKE '%18:00:00+01:00%' THEN 1 END) as updated_to_six_pm_bst,
    COUNT(CASE WHEN scheduled_date NOT LIKE '%18:00:00+01:00%' THEN 1 END) as other_formats
FROM fixtures;

-- Show any fixtures that weren't updated (for review)
SELECT id, scheduled_date, 'Not updated - format not recognized' as note
FROM fixtures 
WHERE scheduled_date NOT LIKE '%18:00:00+01:00%'
ORDER BY scheduled_date;

-- If everything looks correct, COMMIT; otherwise ROLLBACK
COMMIT; 