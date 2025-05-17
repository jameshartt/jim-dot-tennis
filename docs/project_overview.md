# Jim.Tennis Project Overview

## Project Purpose
Jim.Tennis is an internal tool for St Ann's Tennis Club to facilitate team management within the Brighton and Hove Parks League. The application provides a suite of tools for team captains and players to manage availability, plan fixtures, and coordinate match participation.

## Core Goals

1. **Availability Management**
   - Allow players to update their own availability
   - Enable captains to manage availability for players who cannot interact with the tool
   - Create an extremely intuitive and user-friendly experience for updating availability

2. **Team Selection**
   - Support hierarchical team selection process (Division 1 picks first, then Division 2, etc.)
   - Notify captains when "lower" division captains have made their selections
   - Automate and simplify the process of player selection based on availability

3. **Fixture Management**
   - Provide clear scheduling of upcoming fixtures
   - Track match details, locations, and results
   - Streamline communication about matches

4. **Notifications & Communication**
   - Implement push notifications through PWA capabilities
   - Remind players about upcoming fixtures and availability deadlines
   - Facilitate easy sharing of information within existing communication channels (e.g., WhatsApp)

## Technical Approach

1. **Server-Side Rendering**
   - Prioritize server-side rendering whenever possible
   - Use HTMX to minimize client-side JavaScript
   - Create a fast, responsive experience with minimal client-side complexity

2. **Progressive Web App (PWA)**
   - Implement as a PWA to enable push notifications and offline capabilities
   - Ensure mobile-friendly design for easy access on all devices

3. **User Experience**
   - Focus on creating an extremely simple, intuitive interface
   - Design for minimal friction in all user interactions
   - Seamlessly integrate with existing communication workflows (WhatsApp)

## Target Users

1. **Team Captains**
   - Responsible for managing one or more teams
   - Need tools to select players, coordinate fixtures, and manage team communication
   - Require notifications about player availability and selection status

2. **Players**
   - Need simple, easy ways to update availability
   - Require notifications about selection status and upcoming fixtures
   - May vary in technical proficiency (app must be accessible to all skill levels)

## Current Development Status

The current codebase includes models for the Parks League structure:
- Leagues, divisions, seasons
- Clubs, teams, players
- Fixtures, matchups
- Availability tracking
- Messaging and notification systems

The focus is now on implementing a user-friendly interface to make these features accessible and intuitive for all users. 