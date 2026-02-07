# Admin Match Card Import

This document describes the web-based match card import functionality available in the admin panel.

## Overview

The Admin Match Card Import feature provides a user-friendly web interface for importing completed match results from the BHPLTA (Brighton & Hove Parks Lawn Tennis Association) website. This feature utilizes automatic WordPress nonce extraction, eliminating the need for manual browser sessions.

## Access

The match card import functionality is available to admin users through:

- **URL**: `/admin/match-card-import`
- **Navigation**: Admin Dashboard → Results & Standings → "🎾 Match Card Import"

## Features

### 🚀 **Automatic Nonce Extraction**
- No manual browser inspection required
- Real-time WordPress nonce discovery
- Automatic retry on nonce expiration
- Secure handling of club passwords

### 📊 **Per-Week Import**
- Import match cards for individual weeks (1-18)
- Week dropdown selection
- Current week auto-detection based on time of year

### 🔧 **Flexible Configuration**
- **Year Selection**: Current year by default, supports next year
- **Club Settings**: Pre-configured for "St+Anns" (Club ID: 10)
- **Secure Password Input**: Club password field with secure handling
- **Import Options**: Dry run mode and clear existing matchups

### 📈 **Comprehensive Results**
- Real-time processing statistics
- Detailed aggregate reporting
- Error and warning display
- Unmatched player identification
- Processing time tracking

## Usage

### Basic Import Process

1. **Navigate to Import Page**
   - Go to Admin Dashboard
   - Click "🎾 Match Card Import" under Results & Standings

2. **Configure Import Parameters**
   - **Week**: Select week 1-18 from dropdown
   - **Year**: Set import year (defaults to current year)
   - **Club Name**: Pre-set to "St+Anns" 
   - **Club ID**: Pre-set to 10
   - **Club Password**: Enter your club's BHPLTA password
   - **Options**: Choose dry run and/or clear existing data

3. **Execute Import**
   - Click "🚀 Start Import"
   - Monitor real-time progress
   - Review detailed results

### Import Options

#### **Dry Run Mode** (Recommended for first-time use)
- ✅ **Enabled by default**
- Tests import without database changes
- Validates nonce extraction and data parsing
- Shows what would be imported

#### **Clear Existing Matchups**
- ⚠️ **Use with caution**
- Removes existing matchup data before importing
- Useful for re-importing corrected data
- Handles derby matches properly

### Results Display

The import results show comprehensive statistics:

- **Matches Processed**: Total match cards found and processed
- **Fixtures Updated**: Database fixtures updated with match card data
- **Matchups Created/Updated**: Individual match results processed
- **Players Matched**: Players successfully matched to database records
- **Errors**: Any issues encountered during processing

### Error Handling

The system provides detailed error information:

- **Validation Errors**: Invalid parameters or missing data
- **Network Errors**: Connectivity issues with BHPLTA website
- **Authentication Errors**: Invalid club codes or expired nonces
- **Data Errors**: Unmatched players or fixture conflicts

## Technical Details

### Auto-Nonce Extraction Process

1. **Website Access**: Connects to BHPLTA match cards page
2. **HTML Parsing**: Extracts WordPress nonce from JavaScript variables
3. **Pattern Matching**: Handles multiple nonce formats (`my_ajax_object2.nonce`, form fields, data attributes)  
4. **Gzip Handling**: Properly decompresses website responses
5. **Cookie Management**: Uses club code for authenticated access

### Database Integration

- **Repository Pattern**: Uses existing repository interfaces
- **Transaction Safety**: Proper error handling and rollback
- **Data Validation**: Validates all imported data before saving
- **Relationship Management**: Maintains fixture, matchup, and player relationships

### Security Features

- **Password Security**: Club passwords are not stored, only used for requests
- **Input Validation**: All form inputs are validated and sanitized
- **Authentication**: Requires admin role for access
- **CSRF Protection**: Built-in request validation

## Configuration

### Default Settings

```go
// Default configuration values
ClubName: "St+Anns"
ClubID: 10
Year: CurrentYear
DefaultWeek: EstimatedCurrentWeek
BaseURL: "https://www.bhplta.co.uk/wp-admin/admin-ajax.php"
RateLimit: 2 seconds
```

### Customization

Admin users can modify:
- Club name and ID for different clubs
- Year for historical or future imports  
- Week selection for specific periods
- Import options for different scenarios

## Troubleshooting

### Common Issues

1. **"Failed to extract nonce"**
   - Check network connectivity
   - Verify club code is correct and active
   - Ensure BHPLTA website is accessible

2. **"No match cards found"**
   - Week may not have published results yet
   - Club may not have matches in selected week
   - Year/week combination may be invalid

3. **"Unmatched players"**
   - Players in match cards don't exist in database
   - Name differences between systems
   - New players need to be added to database first

4. **"Permission denied"**
   - Requires admin role access
   - Session may have expired
   - Check user authentication

### Support

For technical issues:
1. Check error messages for specific details
2. Try dry run mode first to identify issues
3. Verify club credentials and parameters
4. Review system logs for detailed error information

## Related Documentation

- [Automatic Nonce Extraction](automatic_nonce_extraction.md) - Technical details
- [Command Line Import Tools](../scripts/) - CLI alternatives  
- [Admin Dashboard](technical_implementation_plan.md) - Overall admin system
- [Match Card Data Structure](../internal/services/) - Data models and processing 