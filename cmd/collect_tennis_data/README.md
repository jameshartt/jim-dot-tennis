# Tennis Player Data Collection

This script collects tennis player data for the "what-three-pros" authentication system in the Jim.Tennis application.

## Data Structure

The script generates a JSON file with the following structure:

```json
{
  "last_updated": "2025-06-12T12:10:45Z",
  "atp_players": [
    {
      "id": 1,
      "first_name": "Jannik",
      "last_name": "Sinner", 
      "common_name": "Jannik Sinner",
      "nationality": "Italy",
      "gender": "Male",
      "current_rank": 1,
      "highest_rank": 1,
      "year_pro": 2018,
      "wikipedia_url": "https://en.wikipedia.org/wiki/Jannik_Sinner",
      "hand": "Right-handed",
      "birth_date": "2001-08-16",
      "birth_place": "San Candido, Italy"
    }
  ],
  "wta_players": [...]
}
```

## Features

- **ID Strategy**: ATP players have IDs 1-200, WTA players have IDs 1001-1200
- **Wikipedia Links**: Automatically generates Wikipedia URLs for each player
- **Gender Distinction**: Clear separation between ATP (Male) and WTA (Female) players
- **Rankings**: Current rank based on Tennis Abstract data
- **Biographical Data**: Birth date, birth place, nationality, and professional career start

## Usage

```bash
# Run the data collection script
go run main.go

# Output will be written to tennis_players.json
```

## Data Sources

1. **Primary**: Tennis Abstract (https://www.tennisabstract.com/)
   - ATP Rankings: https://www.tennisabstract.com/reports/atp_elo_ratings.html
   - WTA Rankings: https://www.tennisabstract.com/reports/wta_elo_ratings.html

2. **Wikipedia**: Auto-generated URLs for player biographical information

## Current Status

- ✅ JSON structure defined
- ✅ Sample data with top 5 ATP and WTA players
- ⏳ Full Tennis Abstract scraping (ready for implementation)
- ⏳ Data validation and error handling
- ⏳ Automated updates

## For "What-Three-Pros" Authentication

This data will be used to generate memorable authentication phrases like:
- "Sinner-Gauff-Djokovic"
- "Sabalenka-Alcaraz-Swiatek"

The system will:
1. Load this JSON data into the application database
2. Generate random 3-player combinations for user authentication
3. Allow users to access their accounts using these tennis pro combinations instead of passwords

## Future Enhancements

- [ ] Full data collection from Tennis Abstract (top 100 each)
- [ ] Data validation and cleanup
- [ ] Automatic periodic updates
- [ ] Historical ranking data
- [ ] Player photos/avatars integration 