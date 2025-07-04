# Tennis Import Credentials Setup

This guide explains how to get the required authentication credentials for importing tennis match card data.

## Required Credentials

You only need one piece of authentication data:

- **TENNIS_NONCE** - The BHPLTA nonce for API authentication

## Getting Your Credentials

### 1. Login to the Tennis Club Website

1. Go to the tennis club website
2. Login to your account
3. Navigate to the match cards section

### 2. Get the Nonce

**Using Browser Developer Tools:**

1. Open your browser's Developer Tools (F12)
2. Go to the Network tab
3. Make any request that uses the nonce (e.g., change week selection)
4. Look for requests to `admin-ajax.php`
5. Check the request payload for the `nonce` parameter
6. Copy the nonce value

**Alternative - Check Page Source:**

1. Right-click on the match cards page and "View Source"
2. Search for `"nonce"` in the page source
3. Look for a JavaScript variable or hidden input containing the nonce
4. Copy the nonce value

## Setting Up Credentials

### Using the Tennis Import Script (Recommended)

```bash
./scripts/tennis-import.sh setup
```

This will prompt you to enter your nonce and save it securely.

### Manual Setup

Create a `.tennis-credentials` file in the scripts directory:

```bash
# Tennis Import Credentials
export TENNIS_NONCE='your-nonce-here'
```

## Testing Your Setup

Test that your credentials work:

```bash
./scripts/tennis-import.sh run-dry
```

This will run a dry-run import to verify your credentials without making database changes.

## Security Notes

- Keep your credentials secure and don't share them
- The nonce may expire periodically - if imports start failing, get a fresh nonce
- The `.tennis-credentials` file is excluded from git to prevent accidental commits

## Troubleshooting

### "Permission Denied" errors

This usually means:
- Your nonce has expired - get a fresh one
- You're not logged in to the tennis club website

### "No match cards found"

This can happen if:
- The week you're trying to import doesn't have data yet
- Your club/season settings are incorrect
- There's an authentication issue 