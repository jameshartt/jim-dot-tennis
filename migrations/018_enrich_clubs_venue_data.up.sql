-- Migration 018: Enrich clubs table with venue data
-- Adds structured venue fields for court info, location, transport, and tips

ALTER TABLE clubs ADD COLUMN latitude REAL;
ALTER TABLE clubs ADD COLUMN longitude REAL;
ALTER TABLE clubs ADD COLUMN postcode TEXT;
ALTER TABLE clubs ADD COLUMN address_line_1 TEXT;
ALTER TABLE clubs ADD COLUMN address_line_2 TEXT;
ALTER TABLE clubs ADD COLUMN city TEXT;
ALTER TABLE clubs ADD COLUMN court_surface TEXT;
ALTER TABLE clubs ADD COLUMN court_count INTEGER;
ALTER TABLE clubs ADD COLUMN parking_info TEXT;
ALTER TABLE clubs ADD COLUMN transport_info TEXT;
ALTER TABLE clubs ADD COLUMN tips TEXT;
ALTER TABLE clubs ADD COLUMN google_maps_url TEXT;
