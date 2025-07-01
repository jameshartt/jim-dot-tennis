# Credentials Setup for Match Card Import Scripts

This project contains scripts that require sensitive authentication data (cookies, nonces, etc.) to access the BHPLTA website. To keep this sensitive data out of version control, we use template files.

## Setup Instructions

### 1. Create your credential scripts

Copy the template files to create your working scripts:

```bash
cd scripts/
cp test_matchcard_api.template.sh test_matchcard_api.sh
cp import_all_weeks.template.sh import_all_weeks.sh
```

### 2. Fill in your actual credentials

Edit the newly created files and replace the placeholder values:

- `YOUR_NONCE_HERE` - The WordPress nonce from your browser session
- `YOUR_CLUB_CODE_HERE` - Your club code cookie value  
- `YOUR_WP_LOGGED_IN_COOKIE_HERE` - Your WordPress logged-in cookie
- `YOUR_WP_SEC_COOKIE_HERE` - Your WordPress security cookie

### 3. Make scripts executable

```bash
chmod +x test_matchcard_api.sh
chmod +x import_all_weeks.sh
```

## Getting Your Credentials

### Using Browser Developer Tools

1. Open the BHPLTA website in your browser and log in
2. Navigate to a match card page
3. Open Developer Tools (F12)
4. Go to the Network tab
5. Look for AJAX requests to `admin-ajax.php`
6. Check the request headers and form data for the values you need

### Cookie Values

Look for these cookies in your browser:
- `clubcode` - Your club code
- `wordpress_logged_in_*` - Your logged-in session cookie
- `wordpress_sec_*` - Your security cookie

### Nonce Value

The nonce is typically found in:
- AJAX request form data
- Hidden form fields on the page
- JavaScript variables in the page source

## Security Notes

⚠️ **Important Security Information:**

- The actual credential files (`*.sh`) are excluded from git via `.gitignore`
- Never commit files containing real passwords or session tokens
- These credentials may expire and need to be refreshed periodically
- Only share template files (`.template.sh`) with others

## Usage

Once set up, you can run:

```bash
# From the scripts directory:
cd scripts/

# Test a single week
./test_matchcard_api.sh

# Import all weeks
./import_all_weeks.sh

# Import specific week range
./import_all_weeks.sh --start-week=1 --end-week=5

# Dry run (no database changes)
./import_all_weeks.sh --dry-run
```

Or from the project root:

```bash
# Test a single week
scripts/test_matchcard_api.sh

# Import all weeks
scripts/import_all_weeks.sh

# Import specific week range
scripts/import_all_weeks.sh --start-week=1 --end-week=5
``` 