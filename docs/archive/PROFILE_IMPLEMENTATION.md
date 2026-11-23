# Player Profile Implementation - Testing Guide

## Implementation Summary

The player profile view has been successfully implemented following the plan WI-001. This feature provides players with a comprehensive view of their profile information, team assignments, upcoming fixtures, and availability statistics.

## Files Created

1. **internal/players/profile.go** (3,079 bytes)
   - ProfileHandler struct with HandleProfile and handleProfileGet methods
   - Follows the same pattern as AvailabilityHandler
   - Uses token-based authentication

2. **templates/players/profile.html** (14,476 bytes)
   - Standalone HTML template with mobile-responsive design
   - Green color scheme matching availability page (#2c5530, #4a7c59, #e8f5e8)
   - Card-based layout with sections for:
     - Player header with name and club
     - Current season teams
     - Upcoming fixtures
     - Availability statistics
     - Historical teams (collapsible)

## Files Modified

1. **internal/players/service.go**
   - Added `PlayerProfileData` struct
   - Added `TeamWithDetails` struct
   - Added `AvailabilityStats` struct
   - Added `GetPlayerProfileData(playerID string)` method
   - Added `buildTeamDetails(...)` helper method
   - Added `calculateAvailabilityStats(...)` helper method

2. **internal/players/handler.go**
   - Added `profile *ProfileHandler` field to Handler struct
   - Initialized ProfileHandler in `New()` function
   - Registered `/my-profile/` route with RequireFantasyTokenAuth middleware

## Testing Instructions

### Prerequisites
- Ensure Go 1.24+ is installed
- Database should be populated with test data

### Build and Run
```bash
# Build the application
make build-local

# Run locally (creates database at ./tennis.db)
make run-local

# Server will run at http://localhost:8080
```

### Manual Testing Steps

#### 1. Get a Valid Token
First, navigate to an availability page to get a valid auth token:
```
http://localhost:8080/my-availability/{token}
```

Use the same token format as the availability page (e.g., `Sabalenka_Djokovic_Gauff_Sinner`)

#### 2. Navigate to Profile Page
Replace the `/my-availability/` with `/my-profile/`:
```
http://localhost:8080/my-profile/{same-token}
```

#### 3. Verify Page Sections

**Player Header:**
- [ ] Player name displays correctly (with preferred name if set)
- [ ] Full name shows below if preferred name is used
- [ ] Club name displays
- [ ] "Back to Availability" link works

**Current Season Teams:**
- [ ] All current teams are displayed
- [ ] Team name, division, captains, and roster count show correctly
- [ ] "Team Captain" badge appears if player is a captain
- [ ] Cards have proper styling with hover effects

**Upcoming Fixtures:**
- [ ] Fixtures table displays with date, division, and match details
- [ ] HOME/AWAY/DERBY badges show correctly
- [ ] Fixture data is accurate
- [ ] Empty state if no fixtures

**Availability Statistics:**
- [ ] Availability percentage calculates correctly
- [ ] Day counts (Available, If Needed, Unavailable) are accurate
- [ ] Statistics are based on last 28 days

**Historical Teams:**
- [ ] Section is collapsible (click to expand/collapse)
- [ ] Historical teams exclude current teams
- [ ] Season information displays if available
- [ ] Empty state if no historical teams

#### 4. Test Responsive Design

**Desktop (1024px+):**
- [ ] Teams grid shows multiple columns
- [ ] All sections have proper spacing
- [ ] Cards are appropriately sized

**Tablet (768px):**
- [ ] Teams grid shows 2 columns or single column
- [ ] Fixtures table remains readable
- [ ] Stats grid adjusts to 2 columns

**Mobile (480px):**
- [ ] All content stacks vertically
- [ ] Font sizes reduce appropriately
- [ ] Touch targets are adequate
- [ ] No horizontal scrolling

#### 5. Test Edge Cases

**New Player (No Teams):**
- [ ] Empty states display gracefully
- [ ] No errors in console
- [ ] Page still renders header and availability stats

**Player with Multiple Teams (6+ teams):**
- [ ] All teams display correctly
- [ ] Grid wraps appropriately
- [ ] Page performance is acceptable

**Player as Captain:**
- [ ] "Team Captain" badge displays
- [ ] Player's name in captains list

**No Upcoming Fixtures:**
- [ ] Empty state or no fixtures section
- [ ] No JavaScript errors

**No Availability Data:**
- [ ] Statistics show zeros
- [ ] No errors occur

#### 6. Authentication Testing

**Invalid Token:**
- [ ] Returns 401 Unauthorized or appropriate error
- [ ] Does not crash the application

**Missing Token:**
- [ ] Returns 400 Bad Request
- [ ] Error message is clear

**Valid Token:**
- [ ] Player context extracted correctly
- [ ] All data loads for the correct player

#### 7. Performance Verification

- [ ] Page loads within 2 seconds
- [ ] No N+1 query issues (check logs)
- [ ] Database queries are efficient
- [ ] Template renders without errors

#### 8. Browser Console Check

- [ ] No JavaScript errors
- [ ] No 404 errors for assets
- [ ] No CORS errors

### Repository Methods Used

The implementation reuses existing repository methods:
- `PlayerRepository.FindByID(playerID)`
- `PlayerRepository.FindTeamsForPlayer(playerID, seasonID)`
- `PlayerRepository.FindAllTeamsForPlayer(playerID)`
- `ClubRepository.FindByID(clubID)`
- `SeasonRepository.FindActive()`
- `SeasonRepository.FindByID(seasonID)`
- `TeamRepository.FindByID(teamID)`
- `TeamRepository.FindCaptainsInTeam(teamID, seasonID)`
- `TeamRepository.FindPlayersInTeam(teamID, seasonID)`
- `DivisionRepository.FindByID(divisionID)`
- `Service.GetPlayerUpcomingFixtures(playerID)` (existing)
- `Service.GetPlayerAvailabilityData(playerID)` (existing)

### Code Quality Checklist

- [x] Follows existing code patterns (AvailabilityHandler)
- [x] Uses token-based authentication consistently
- [x] Reuses existing service methods (DRY principle)
- [x] Error handling is graceful (logs errors but doesn't crash)
- [x] Template functions used correctly (formatDate, currentYear)
- [x] Mobile-responsive design implemented
- [x] Consistent styling with availability page
- [x] No hardcoded values
- [x] Proper separation of concerns (Service/Handler/Template)

## Expected Behavior

### Success Case
1. User navigates to `/my-profile/{valid-token}`
2. Page loads within 2 seconds
3. All sections render with appropriate data
4. Mobile view is fully responsive
5. No console errors
6. Navigation links work correctly

### Error Cases
1. **Invalid token** → 401 error page
2. **Missing token** → 400 error page
3. **Database error** → 500 error with fallback HTML
4. **Template error** → Fallback HTML with basic information

## Next Steps

After successful testing:
1. Test with real user data
2. Gather user feedback on design/layout
3. Consider adding additional features:
   - Match history/results
   - Player statistics
   - Team performance metrics
   - Season comparisons

## Troubleshooting

### Issue: Template not found
- Check `templateDir` is correctly set in handler initialization
- Verify `templates/players/profile.html` exists

### Issue: Player not found
- Verify token is valid
- Check middleware is applied correctly
- Verify player exists in database

### Issue: Empty data sections
- Check database has appropriate test data
- Verify repository methods return data
- Check service layer error logs

### Issue: Styling issues
- Clear browser cache
- Verify template is being served correctly
- Check for CSS conflicts

## Architecture Notes

### Why Token-Based Routing?
- Consistent with `/my-availability/{token}` pattern
- No new authentication infrastructure needed
- Leverages existing `RequireFantasyTokenAuth()` middleware
- Secure per-player tokens

### Why Reuse Service Methods?
- DRY (Don't Repeat Yourself) principle
- Single source of truth for data
- Easier maintenance and testing
- Acceptable performance for page loads

### Why Standalone Template?
- Matches existing player template pattern
- Complete control over mobile responsiveness
- No layout inheritance complexity
- Self-contained and portable

## Implementation Complete ✅

All tasks from WI-001 have been successfully implemented:
- ✅ Service layer data structures and methods
- ✅ Profile handler with routing
- ✅ Profile template with responsive design
- ✅ Route registration with authentication
- ⏳ Manual testing (requires Go environment)
