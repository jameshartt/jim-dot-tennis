-- =============================================================================
-- Seed Script: Brighton & Hove Parks League Tennis Club Venue Data
-- =============================================================================
--
-- Purpose:  Populates enriched venue fields (lat/lon, address, courts, parking,
--           transport, tips, Google Maps links) for all Brighton & Hove Parks
--           League tennis clubs.
--
-- Schema:   Requires migration 018_enrich_clubs_venue_data (adds latitude,
--           longitude, postcode, address_line_1, address_line_2, city,
--           court_surface, court_count, parking_info, transport_info, tips,
--           google_maps_url columns to the clubs table).
--
-- Safety:   This script is idempotent. It uses UPDATE ... WHERE name LIKE ...
--           so it can be run multiple times without duplicating data. It only
--           modifies existing club rows -- it will never INSERT new clubs.
--           LIKE patterns are used to safely match club names regardless of
--           minor variations (e.g. apostrophe encoding differences).
--
-- Usage:    sqlite3 tennis.db < scripts/seed_club_venue_data.sql
--
-- Author:   Seed data for Brighton & Hove parks tennis venues
-- =============================================================================

BEGIN TRANSACTION;

-- =============================================================================
-- 1. St Ann's Well Gardens Tennis Club
-- =============================================================================
-- Located in St Ann's Well Gardens park in Hove. The club's courts sit in a
-- beautiful landscaped park setting, accessed from the Nizells Avenue entrance.
-- Well served by bus routes along Church Road and a short walk from Hove station.
UPDATE clubs SET
    latitude        = 50.8386,
    longitude       = -0.1670,
    postcode        = 'BN3 1PJ',
    address_line_1  = 'St Ann''s Well Gardens',
    address_line_2  = 'Nizells Avenue',
    city            = 'Hove',
    court_surface   = 'Hard',
    court_count     = 4,
    parking_info    = 'Limited street parking on Nizells Ave and surrounding streets. Church Road car parks nearby.',
    transport_info  = 'Bus: 1, 1A, 6 (Nizells Avenue). 5 min walk from Hove station.',
    tips            = 'Courts in beautiful park setting. Enter via Nizells Avenue gate. Clubhouse has changing facilities.',
    google_maps_url = 'https://maps.google.com/?q=50.8386,-0.1670',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%St Ann%';

-- =============================================================================
-- 2. Blakers Park Tennis Club
-- =============================================================================
-- Situated in Blakers Park in the Preston Park area of Brighton, accessed from
-- Cleveland Road. A pleasant neighbourhood park with good bus connections via
-- Preston Drove and Ditchling Road.
UPDATE clubs SET
    latitude        = 50.8418,
    longitude       = -0.1378,
    postcode        = 'BN1 6FF',
    address_line_1  = 'Blakers Park',
    address_line_2  = 'Cleveland Road',
    city            = 'Brighton',
    court_surface   = 'Hard',
    court_count     = 3,
    parking_info    = 'Street parking on Cleveland Road and surrounding streets.',
    transport_info  = 'Bus: 5, 5A (Preston Drove), 50 (Ditchling Road).',
    tips            = 'Courts in Blakers Park, accessed from Cleveland Road entrance.',
    google_maps_url = 'https://maps.google.com/?q=50.8418,-0.1378',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Blakers%';

-- =============================================================================
-- 3. BLAGSS Tennis Club
-- =============================================================================
-- Brighton & Lewes & Gatwick Squash & Sports -- an LGBTQ+ friendly sports club.
-- Typically uses Dyke Road Park courts but venue may vary, so players should
-- check the club website for current playing location.
UPDATE clubs SET
    latitude        = 50.8390,
    longitude       = -0.1519,
    postcode        = 'BN1 3JA',
    address_line_1  = 'Dyke Road Park',
    address_line_2  = 'Dyke Road',
    city            = 'Brighton',
    court_surface   = 'Hard',
    court_count     = NULL,
    parking_info    = 'Street parking near Dyke Road Park.',
    transport_info  = 'Bus: 27 (Dyke Road).',
    tips            = 'BLAGSS is an LGBTQ+ friendly sports club. Check club website for current playing venue.',
    google_maps_url = 'https://maps.google.com/?q=50.8390,-0.1519',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%BLAGSS%';

-- =============================================================================
-- 4. Dyke Park Tennis Club
-- =============================================================================
-- Based at Dyke Road Park in central Brighton. The park stretches up a hill and
-- the courts are located towards the top, so allow a few minutes to walk up from
-- the main Dyke Road entrance.
UPDATE clubs SET
    latitude        = 50.8390,
    longitude       = -0.1519,
    postcode        = 'BN1 3JA',
    address_line_1  = 'Dyke Road Park',
    address_line_2  = 'Dyke Road',
    city            = 'Brighton',
    court_surface   = 'Hard',
    court_count     = 3,
    parking_info    = 'Limited street parking on Dyke Road. Some spaces in park car park.',
    transport_info  = 'Bus: 27 (Dyke Road), 46, 77.',
    tips            = 'Enter park from Dyke Road entrance. Courts are towards the top of the park.',
    google_maps_url = 'https://maps.google.com/?q=50.8390,-0.1519',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Dyke%';

-- =============================================================================
-- 5. Hollingbury Park Tennis Club
-- =============================================================================
-- Located in Hollingbury Park on the northern edge of Brighton. The park has its
-- own free car park which is convenient for players driving from further afield.
-- Well served by buses along Ditchling Road.
UPDATE clubs SET
    latitude        = 50.8559,
    longitude       = -0.1340,
    postcode        = 'BN1 8GA',
    address_line_1  = 'Hollingbury Park',
    address_line_2  = 'Ditchling Road',
    city            = 'Brighton',
    court_surface   = 'Hard',
    court_count     = 3,
    parking_info    = 'Free car park in Hollingbury Park.',
    transport_info  = 'Bus: 5, 5A (Ditchling Road), alight at Hollingbury Park.',
    tips            = 'Follow signs to tennis courts from main park entrance on Ditchling Road.',
    google_maps_url = 'https://maps.google.com/?q=50.8559,-0.1340',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Hollingbury%';

-- =============================================================================
-- 6. Hove Park Tennis Club
-- =============================================================================
-- In Hove Park, a large park in west Hove. Best accessed from the Goldstone
-- Crescent entrance which is closest to the courts and the free car park.
-- A Beryl bike dock is located near the Old Shoreham Road entrance.
UPDATE clubs SET
    latitude        = 50.8444,
    longitude       = -0.1815,
    postcode        = 'BN3 6HP',
    address_line_1  = 'Hove Park',
    address_line_2  = 'Old Shoreham Road',
    city            = 'Hove',
    court_surface   = 'Hard',
    court_count     = 4,
    parking_info    = 'Free car park off Goldstone Crescent entrance.',
    transport_info  = 'Bus: 6 (Old Shoreham Road), Hove Park stop.',
    tips            = 'Enter from Goldstone Crescent for easiest access to courts. Beryl bike dock near Old Shoreham Road entrance.',
    google_maps_url = 'https://maps.google.com/?q=50.8444,-0.1815',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Hove Park%';

-- =============================================================================
-- 7. King Alfred Tennis Club
-- =============================================================================
-- Courts near the seafront by the King Alfred Leisure Centre site in Hove.
-- Good bus connections along the Kingsway seafront route and Church Road.
-- Exposed coastal location means wind can be a factor -- bring layers!
UPDATE clubs SET
    latitude        = 50.8330,
    longitude       = -0.1750,
    postcode        = 'BN3 2WW',
    address_line_1  = 'King Alfred Leisure Centre area',
    address_line_2  = 'Kingsway',
    city            = 'Hove',
    court_surface   = 'Hard',
    court_count     = 2,
    parking_info    = 'King Alfred car park (paid), street parking on Western Esplanade.',
    transport_info  = 'Bus: 1, 1A, 6 (Kingsway/seafront), 2 (Church Road).',
    tips            = 'Courts near the seafront by the King Alfred development site. Can be windy - bring layers!',
    google_maps_url = 'https://maps.google.com/?q=50.8330,-0.1750',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%King Alfred%';

-- =============================================================================
-- 8. Park Avenue Tennis Club
-- =============================================================================
-- A smaller, more intimate club in a residential area of Hove. Courts are
-- located directly on Park Avenue itself. Close to Hove town centre with
-- good bus connections.
UPDATE clubs SET
    latitude        = 50.8380,
    longitude       = -0.1720,
    postcode        = 'BN3',
    address_line_1  = 'Park Avenue',
    address_line_2  = NULL,
    city            = 'Hove',
    court_surface   = 'Hard',
    court_count     = 2,
    parking_info    = 'Street parking on Park Avenue and surrounding residential streets.',
    transport_info  = 'Bus: 1, 1A (close to Hove town centre).',
    tips            = 'Smaller club in a residential area. Courts on Park Avenue itself.',
    google_maps_url = 'https://maps.google.com/?q=50.8380,-0.1720',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Park Avenue%';

-- =============================================================================
-- 9. Preston Park Tennis Club
-- =============================================================================
-- The largest park court setup in the league, with 6 hard courts adjacent to the
-- velodrome in Preston Park. Excellent transport links: Preston Park station is a
-- 5-minute walk and multiple bus routes serve Preston Road and London Road.
-- Free parking available in the park car park off The Drove.
UPDATE clubs SET
    latitude        = 50.8467,
    longitude       = -0.1468,
    postcode        = 'BN1 6SD',
    address_line_1  = 'Preston Park',
    address_line_2  = 'Preston Road',
    city            = 'Brighton',
    court_surface   = 'Hard',
    court_count     = 6,
    parking_info    = 'Preston Park car park (free, off The Drove). London Road retail park nearby.',
    transport_info  = 'Bus: 5, 5A, 49 (Preston Road), London Road stops nearby. Preston Park station is a 5-minute walk.',
    tips            = 'Largest park court setup in the league. Courts adjacent to the velodrome. Beryl bike dock at Preston Park.',
    google_maps_url = 'https://maps.google.com/?q=50.8467,-0.1468',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Preston Park%';

-- =============================================================================
-- 10. Queens Park Tennis Club
-- =============================================================================
-- Located in Queens Park in the Hanover/Kemptown area of Brighton. A beautiful
-- Victorian park with a cafe for refreshments. Enter from Queens Park Road.
UPDATE clubs SET
    latitude        = 50.8337,
    longitude       = -0.1232,
    postcode        = 'BN2 9ZF',
    address_line_1  = 'Queens Park',
    address_line_2  = 'Queens Park Road',
    city            = 'Brighton',
    court_surface   = 'Hard',
    court_count     = 3,
    parking_info    = 'Limited street parking on Queens Park Road, some in park.',
    transport_info  = 'Bus: 7 (Queens Park Road), 14, 21.',
    tips            = 'Beautiful park setting. Enter from Queens Park Road. Cafe in the park for refreshments.',
    google_maps_url = 'https://maps.google.com/?q=50.8337,-0.1232',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Queens Park%';

-- =============================================================================
-- 11. Rookery Tennis Club
-- =============================================================================
-- A small, intimate venue in The Rookery park in the Hanover area. Can be
-- tricky to find -- look for the park entrance on Park Crescent Terrace.
-- Very limited parking due to resident permit zones, so public transport is
-- recommended.
UPDATE clubs SET
    latitude        = 50.8395,
    longitude       = -0.1190,
    postcode        = 'BN2 3HF',
    address_line_1  = 'The Rookery',
    address_line_2  = 'Park Crescent Terrace',
    city            = 'Brighton',
    court_surface   = 'Hard',
    court_count     = 2,
    parking_info    = 'Very limited street parking. Resident permit area - check signage.',
    transport_info  = 'Bus: 23, 24 (Freshfield Road), 2 (Elm Grove).',
    tips            = 'Small intimate setting in The Rookery park. Can be hard to find - look for park entrance on Park Crescent Terrace.',
    google_maps_url = 'https://maps.google.com/?q=50.8395,-0.1190',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Rookery%';

-- =============================================================================
-- 12. Saltdean Tennis Club
-- =============================================================================
-- The furthest east club in the league, located in Saltdean Park near the
-- famous Saltdean Lido. Allow extra travel time from central Brighton as bus
-- journeys take 20-30 minutes along the coast road. Free parking available.
UPDATE clubs SET
    latitude        = 50.7985,
    longitude       = -0.0335,
    postcode        = 'BN2 8HA',
    address_line_1  = 'Saltdean Park',
    address_line_2  = 'Saltdean Vale',
    city            = 'Saltdean',
    court_surface   = 'Hard',
    court_count     = 2,
    parking_info    = 'Free parking near Saltdean Lido / Saltdean Park.',
    transport_info  = 'Bus: 12, 12A, 14 (Coast Road to Saltdean), alight at Saltdean Lido.',
    tips            = 'Furthest east club in the league. Near Saltdean Lido. Allow extra travel time from central Brighton (20-30 mins by bus).',
    google_maps_url = 'https://maps.google.com/?q=50.7985,-0.0335',
    updated_at      = CURRENT_TIMESTAMP
WHERE name LIKE '%Saltdean%';

-- =============================================================================
-- Verification: Show all clubs and their updated venue data
-- =============================================================================
SELECT
    name,
    latitude,
    longitude,
    postcode,
    address_line_1,
    city,
    court_surface,
    court_count,
    CASE
        WHEN parking_info IS NOT NULL THEN 'Yes'
        ELSE 'No'
    END AS has_parking_info,
    CASE
        WHEN transport_info IS NOT NULL THEN 'Yes'
        ELSE 'No'
    END AS has_transport_info,
    CASE
        WHEN tips IS NOT NULL THEN 'Yes'
        ELSE 'No'
    END AS has_tips
FROM clubs
ORDER BY name;

COMMIT;
