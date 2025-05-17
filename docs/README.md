# Jim.Tennis Documentation

This directory contains various documentation for the Jim.Tennis application - an internal tool for St Ann's Tennis Club to manage team selection, player availability, and fixture coordination within the Brighton and Hove Parks League.

## Project Documentation

- [Project Overview](./project_overview.md) - Goals, purpose, and core requirements of the system
- [User Experience Requirements](./user_experience_requirements.md) - Detailed requirements for the application's UX
- [Technical Implementation Plan](./technical_implementation_plan.md) - Approach for building and implementing the application
- [Docker Setup](./docker_setup.md) - Instructions for using Docker with this project
- [DigitalOcean Deployment](./digitalocean_deployment.md) - Guide for deploying to DigitalOcean
- [DigitalOcean Monitoring](./digitalocean_monitoring.md) - Advanced monitoring and management on DigitalOcean

## Database Documentation

The following files provide visualization options for the database schema:

### Graphviz Visualization

The `database_erd.dot` file can be rendered using Graphviz:

```bash
# Generate PNG image
dot -Kdot -Tpng docs/database_erd.dot -o docs/images/database_erd.png

# For SVG output
dot -Kdot -Tsvg docs/database_erd.dot -o docs/images/database_erd.svg
```

### PlantUML Visualization

The `database_erd.puml` file can be rendered using the PlantUML tool or online services:

1. Install PlantUML (requires Java): 
   ```bash
   sudo apt-get install plantuml
   ```

2. Generate diagram:
   ```bash
   plantuml docs/database_erd.puml
   ```

3. Or use the online service: https://www.plantuml.com/plantuml/

### DBDiagram.io Visualization

The `database_erd.dbml` file can be used with the dbdiagram.io website:

1. Go to https://dbdiagram.io/
2. Create a new diagram
3. Copy and paste the contents of `database_erd.dbml` into the editor

This will give you an interactive visualization that you can customize further.

## Technical Architecture

- Data models and their relationships are defined in the `internal/models` directory
- Database migrations are maintained in the `migrations` directory
- The application focuses on server-side rendering with minimal JavaScript (using HTMX)
- Progressive Web App (PWA) capabilities are implemented for push notifications