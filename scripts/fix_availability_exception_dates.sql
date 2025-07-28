-- Fix availability exception end dates to cover full day
-- Update all exceptions where start_date = end_date to extend end_date by one day

-- This ensures that single-day exceptions properly cover the entire day
-- instead of ending at the same timestamp they start

UPDATE player_availability_exceptions 
SET 
    end_date = datetime(date(start_date), '+1 day'),
    updated_at = CURRENT_TIMESTAMP
WHERE 
    date(start_date) = date(end_date)
    AND time(start_date) = time(end_date); 