#!/bin/bash

# Test the BHPLTA match card API with the provided parameters
# Copy this file to test_matchcard_api.sh and fill in the real values

# Uncomment and use the curl command if needed for testing:
# curl 'https://www.bhplta.co.uk/wp-admin/admin-ajax.php' \
#   --compressed \
#   -X POST \
#   -H 'User-Agent: Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0' \
#   -H 'Accept: */*' \
#   -H 'Accept-Language: en-GB,en;q=0.5' \
#   -H 'Accept-Encoding: gzip, deflate, br, zstd' \
#   -H 'Content-Type: application/x-www-form-urlencoded; charset=UTF-8' \
#   -H 'X-Requested-With: XMLHttpRequest' \
#   -H 'Origin: https://www.bhplta.co.uk' \
#   -H 'Connection: keep-alive' \
#   -H 'Referer: https://www.bhplta.co.uk/bhplta_tables/parks-league-match-cards/?id=3356' \
#   -H 'Cookie: wordpress_sec_d9e736f9c59ae0b57f0c59c5392dc843=YOUR_WP_SEC_COOKIE; clubcode=YOUR_CLUB_CODE; wordpress_test_cookie=WP%20Cookie%20check; wordpress_logged_in_d9e736f9c59ae0b57f0c59c5392dc843=YOUR_WP_LOGGED_IN_COOKIE' \
#   -H 'Sec-Fetch-Dest: empty' \
#   -H 'Sec-Fetch-Mode: cors' \
#   -H 'Sec-Fetch-Site: same-origin' \
#   -H 'Priority: u=0' \
#   -H 'TE: trailers' \
#   --data-raw 'nonce=YOUR_NONCE&action=bhplta_club_scores_get_scores_week_change&selected_week=1&year=2025&club_id=10&club_name=St+Anns&passcode=' \
#   --output response2.json

# Replace the placeholders below with your actual values:
NONCE="YOUR_NONCE_HERE"
CLUB_CODE="YOUR_CLUB_CODE_HERE"
WP_LOGGED_IN="YOUR_WP_LOGGED_IN_COOKIE_HERE"
WP_SEC="YOUR_WP_SEC_COOKIE_HERE"

go run ../cmd/import-matchcards/main.go \
  -db="../tennis.db" \
  -nonce="$NONCE" \
  -club-code="$CLUB_CODE" \
  -wp-logged-in="$WP_LOGGED_IN" \
  -wp-sec="$WP_SEC" \
  -week=11 \
  -year=2025 \
  -club-id=10 \
  -club-name="St Anns" \
  -verbose 