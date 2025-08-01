# Automatic Nonce Extraction

This document describes the new automatic nonce extraction functionality that eliminates the need to manually obtain WordPress nonces from the BHPLTA website.

## Overview

Previously, importing match card data required manually extracting a nonce (CSRF token) from the BHPLTA website using browser developer tools. The new automatic nonce extraction feature automates this process by:

1. **Scraping the BHPLTA website** to find the nonce embedded in the HTML/JavaScript
2. **Parsing multiple nonce patterns** to handle different WordPress configurations  
3. **Managing nonce expiration** by re-extracting when needed
4. **Providing fallback options** if automatic extraction fails

## How It Works

### Technical Implementation

The nonce extractor works by:

1. Making an HTTP request to the BHPLTA match cards page
2. Parsing the HTML response with goquery
3. Looking for the nonce in multiple locations:
   - JavaScript variables (e.g., `my_ajax_object2.nonce`)
   - Hidden form fields with nonce-related names
   - HTML data attributes (`data-nonce`, `data-wp-nonce`)
4. Returning the nonce with an estimated expiration time

### WordPress Nonce Patterns

The extractor searches for these common WordPress nonce patterns:

```javascript
// Pattern 1: my_ajax_object2 object
my_ajax_object2 = {"nonce":"abc123def456",...}

// Pattern 2: Direct nonce variable  
var nonce = "abc123def456";

// Pattern 3: WordPress localized script
wp_nonce: "abc123def456"
```

```html
<!-- Pattern 4: Hidden form fields -->
<input type="hidden" name="_wpnonce" value="abc123def456" />

<!-- Pattern 5: Data attributes -->
<div data-nonce="abc123def456"></div>
```

## Usage

### 1. Extract Nonce Only

Test nonce extraction without importing:

```bash
# Build the utility
make build-extract-nonce

# Extract nonce without club code
./bin/extract-nonce -verbose

# Extract nonce with club code (recommended)
./bin/extract-nonce -club-code="STANN001" -verbose
```

Output:
```
Extracting nonce from BHPLTA website...
Successfully extracted nonce!
Nonce: abc123def456789
Expires at: 2024-01-15 14:30:00
Club code: STANN001
Full nonce length: 15 characters
```

### 2. Import with Auto-Nonce

Import match cards with automatic nonce extraction:

```bash
# Build the utility
make build-import-matchcards

# Import with auto-nonce flag
./bin/import-matchcards \
  -auto-nonce \
  -club-code="STANN001" \
  -week=1 \
  -year=2024 \
  -club-id=123 \
  -club-name="St Ann's Tennis Club" \
  -db="./tennis.db" \
  -verbose \
  -dry-run

# Import without nonce (auto-extracts when nonce is empty)
./bin/import-matchcards \
  -club-code="STANN001" \
  -week=1 \
  -year=2024 \
  # ... other parameters
```

### 3. Manual Nonce (Fallback)

You can still provide a manual nonce if needed:

```bash
./bin/import-matchcards \
  -nonce="manually-extracted-nonce" \
  -club-code="STANN001" \
  # ... other parameters
```

## Integration with Existing Scripts

The automatic nonce extraction is backward compatible with existing scripts. You can update your import scripts to use the new functionality:

### Before (Manual)
```bash
# Required manual nonce extraction
export TENNIS_NONCE="manually-copied-nonce"
./bin/import-matchcards -nonce="$TENNIS_NONCE" ...
```

### After (Automatic)
```bash
# No manual nonce needed
./bin/import-matchcards -auto-nonce -club-code="STANN001" ...

# Or simply omit the nonce (auto-extracts when empty)
./bin/import-matchcards -club-code="STANN001" ...
```

## Error Handling

### Common Issues

1. **Network connectivity problems**
   ```
   Failed to extract nonce: failed to fetch page: context deadline exceeded
   ```
   
2. **Website structure changes**
   ```
   Failed to extract nonce: could not find nonce in page
   ```
   
3. **Invalid club code**
   ```
   Failed to extract nonce: unexpected status code: 403
   ```

### Troubleshooting

If automatic extraction fails:

1. **Check network connectivity**
   ```bash
   curl -I https://www.bhplta.co.uk/bhplta_tables/parks-league-match-cards/
   ```

2. **Verify club code**
   - Ensure the club code is correct and active
   - Try accessing the website manually with the club code

3. **Use verbose output**
   ```bash
   ./bin/extract-nonce -club-code="STANN001" -verbose
   ```

4. **Fall back to manual extraction**
   - Use browser developer tools as before
   - Provide the nonce manually with `-nonce="manual-nonce"`

## Demo Script

Run the demo to see all features in action:

```bash
# Run demo with your club code
./scripts/demo-auto-nonce.sh STANN001

# Demo with specific week/year
./scripts/demo-auto-nonce.sh STANN001 5 2024
```

## Benefits

### For Users
- ✅ **No more manual browser inspection**
- ✅ **No more copying nonces from developer tools**  
- ✅ **Works in automated scripts and CI/CD pipelines**
- ✅ **Automatically handles nonce expiration**
- ✅ **Fallback to manual nonce if needed**

### For Developers
- ✅ **Clean separation of concerns** (nonce extraction vs. data import)
- ✅ **Testable components** with unit tests possible
- ✅ **Extensible patterns** for other WordPress sites
- ✅ **Robust error handling** with multiple fallback strategies

## Implementation Details

### Files Added
- `internal/services/nonce_extractor.go` - Core nonce extraction logic
- `cmd/extract-nonce/main.go` - Command-line nonce extraction utility
- `scripts/demo-auto-nonce.sh` - Demo script showing all features

### Files Modified
- `internal/services/matchcard_service.go` - Added auto-nonce integration
- `cmd/import-matchcards/main.go` - Added `-auto-nonce` flag
- `Makefile` - Added build targets for new utilities
- `scripts/CREDENTIALS_SETUP.md` - Updated with auto-nonce instructions

### Dependencies
The implementation uses existing dependencies:
- `github.com/PuerkitoBio/goquery` - HTML parsing
- Standard library `net/http` - HTTP requests
- Standard library `regexp` - Pattern matching

No new external dependencies were added.

## Future Enhancements

Potential improvements for future versions:

1. **Nonce caching** - Cache valid nonces to reduce website requests
2. **Multiple site support** - Extend to other tennis league websites  
3. **Session management** - Handle login sessions for authenticated access
4. **Rate limiting** - Implement smart rate limiting for nonce requests
5. **Monitoring** - Add metrics and monitoring for nonce extraction success rates 