# Database Visualization Options

This directory contains several files for visualizing the Tennis League database schema in different formats.

## Graphviz Visualization

The `database_erd.dot` file can be rendered using Graphviz:

```bash
# Generate PNG image
dot -Kdot -Tpng docs/database_erd.dot -o docs/images/database_erd.png

# For SVG output
dot -Kdot -Tsvg docs/database_erd.dot -o docs/images/database_erd.svg
```

## PlantUML Visualization

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

## DBDiagram.io Visualization

The `database_erd.dbml` file can be used with the dbdiagram.io website:

1. Go to https://dbdiagram.io/
2. Create a new diagram
3. Copy and paste the contents of `database_erd.dbml` into the editor

This will give you an interactive visualization that you can customize further.

## Compare Your Model Relationships

Use these visualizations to verify:

1. Many-to-many relationships (e.g., League-Season via league_seasons)
2. One-to-many relationships (e.g., Club-Player, Division-Team)
3. Foreign key dependencies
4. Indexes and constraints

The visualizations help spot potential issues such as:
- Circular dependencies
- Missing relationships
- Redundant relationships
- Normalization problems 