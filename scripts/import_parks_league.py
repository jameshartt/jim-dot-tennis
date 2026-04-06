#!/usr/bin/env python3
"""
Import players from Parks League 2026 Excel file into jim.tennis SQLite database.

Reads the xlsx with stdlib only (no openpyxl needed).
Deduplicates against existing players by (first_name, last_name, club_id).
Applies known name corrections.

Usage:
    python3 scripts/import_parks_league.py --dry-run
    python3 scripts/import_parks_league.py
"""

import argparse
import re
import sqlite3
import uuid
import zipfile
import xml.etree.ElementTree as ET
from datetime import datetime


CLUB_ID = 7  # St Ann's

# Known name corrections: (excel_first, excel_last) -> (db_first, db_last) to match existing record
# These map the NEW (Excel) name to the OLD (DB) name so we can find the existing record to update
CORRECTIONS = {
    ("Nix", "Abbott"): {"match": ("Nicola", "Abbott"), "update": {"first_name": "Nix"}},
    ("Will", "Jefferies"): {"match": ("Will", "Jeffries"), "update": {"last_name": "Jefferies"}},
}


def parse_xlsx(filepath):
    """Parse xlsx file using stdlib zipfile + xml. Returns list of (full_name, gender)."""
    z = zipfile.ZipFile(filepath)

    # Read shared strings
    ns = {"s": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
    ss_root = ET.fromstring(z.read("xl/sharedStrings.xml"))
    strings = []
    for si in ss_root.findall(".//s:si", ns):
        text_parts = [t.text for t in si.findall(".//s:t", ns) if t.text]
        strings.append("".join(text_parts))

    # Read sheet data
    sheet_root = ET.fromstring(z.read("xl/worksheets/sheet1.xml"))
    rows = sheet_root.findall(".//s:sheetData/s:row", ns)

    players = []
    for row in rows:
        cells = {}
        for c in row.findall("s:c", ns):
            ref = c.get("r")
            col = re.match(r"([A-Z]+)", ref).group(1)
            typ = c.get("t")
            val_el = c.find("s:v", ns)
            if val_el is not None and val_el.text is not None:
                if typ == "s":
                    cells[col] = strings[int(val_el.text)]
                else:
                    cells[col] = val_el.text

        # Column A = men, Column D = women; row 2 has headers "MEN"/"WOMEN"
        if "A" in cells and cells["A"] not in ("MEN", "WOMEN"):
            players.append((cells["A"], "Men"))
        if "D" in cells and cells["D"] not in ("MEN", "WOMEN"):
            players.append((cells["D"], "Women"))

    return players


def normalize_name(full_name):
    """Normalize whitespace and split into (first_name, last_name)."""
    name = re.sub(r"\s+", " ", full_name.strip())
    parts = name.split(" ", 1)
    if len(parts) == 2:
        return parts[0], parts[1]
    return parts[0], ""


def get_existing_players(db_path):
    """Return dict of (lower_first, lower_last) -> {id, first_name, last_name, gender} for club."""
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    cur = conn.execute(
        "SELECT id, first_name, last_name, gender FROM players WHERE club_id = ?",
        (CLUB_ID,),
    )
    players = {}
    for row in cur:
        key = (row["first_name"].lower(), row["last_name"].lower())
        players[key] = dict(row)
    conn.close()
    return players


def run_import(xlsx_path, db_path, dry_run=True):
    raw_players = parse_xlsx(xlsx_path)
    existing = get_existing_players(db_path)

    # Deduplicate Excel list (Jon Hootman appears in both columns)
    seen = set()
    excel_players = []
    for full_name, gender in raw_players:
        first, last = normalize_name(full_name)
        key = (first.lower(), last.lower())
        if key in seen:
            continue
        seen.add(key)
        excel_players.append((first, last, gender))

    to_insert = []
    to_update = []
    already_exist = []

    for first, last, gender in excel_players:
        key = (first.lower(), last.lower())

        # Check if this is a known correction
        correction = CORRECTIONS.get((first, last))
        if correction:
            match_first, match_last = correction["match"]
            match_key = (match_first.lower(), match_last.lower())
            if match_key in existing:
                player = existing[match_key]
                updates = dict(correction["update"])
                # Also update gender if it differs
                if player["gender"] != gender:
                    updates["gender"] = gender
                to_update.append({
                    "id": player["id"],
                    "old_name": f"{player['first_name']} {player['last_name']}",
                    "new_name": f"{updates.get('first_name', player['first_name'])} {updates.get('last_name', player['last_name'])}",
                    "updates": updates,
                })
                continue
            # Correction target not found — check if new name already exists
            if key in existing:
                already_exist.append(f"{first} {last}")
                continue
            # Neither old nor new name found — treat as new player
            to_insert.append((first, last, gender))
            continue

        # Standard matching
        if key in existing:
            player = existing[key]
            # Check if gender needs updating
            if player["gender"] != gender and player["gender"] == "Unknown":
                to_update.append({
                    "id": player["id"],
                    "old_name": f"{player['first_name']} {player['last_name']}",
                    "new_name": f"{first} {last}",
                    "updates": {"gender": gender},
                })
            else:
                already_exist.append(f"{first} {last}")
            continue

        # New player
        to_insert.append((first, last, gender))

    # Report
    print(f"\n{'=' * 60}")
    print(f"Parks League 2026 Import {'(DRY RUN)' if dry_run else ''}")
    print(f"{'=' * 60}")
    print(f"Excel players (deduplicated): {len(excel_players)}")
    print(f"Existing in DB:               {len(existing)}")
    print()

    print(f"Already exist ({len(already_exist)}):")
    for name in sorted(already_exist):
        print(f"  ✓ {name}")
    print()

    print(f"To update ({len(to_update)}):")
    for u in to_update:
        changes = ", ".join(f"{k}: {v}" for k, v in u["updates"].items())
        print(f"  ~ {u['old_name']} → {u['new_name']}  [{changes}]")
    print()

    print(f"New players to insert ({len(to_insert)}):")
    for first, last, gender in sorted(to_insert, key=lambda x: (x[1], x[0])):
        print(f"  + {first} {last} ({gender})")
    print()

    # Players in DB but not in Excel
    excel_keys = {(f.lower(), l.lower()) for f, l, _ in excel_players}
    # Also include correction match targets as "accounted for"
    for corr in CORRECTIONS.values():
        mf, ml = corr["match"]
        excel_keys.add((mf.lower(), ml.lower()))
    not_in_excel = []
    for key, player in existing.items():
        if key not in excel_keys:
            not_in_excel.append(f"{player['first_name']} {player['last_name']}")
    print(f"In DB but not in Excel ({len(not_in_excel)}) — NO ACTION TAKEN:")
    for name in sorted(not_in_excel):
        print(f"  ? {name}")
    print()

    if dry_run:
        print("Dry run complete. Run without --dry-run to apply changes.")
        return

    # Apply changes
    conn = sqlite3.connect(db_path)
    now = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S")

    # Updates
    for u in to_update:
        set_clauses = ", ".join(f"{k} = ?" for k in u["updates"])
        values = list(u["updates"].values()) + [now, u["id"]]
        conn.execute(
            f"UPDATE players SET {set_clauses}, updated_at = ? WHERE id = ?",
            values,
        )

    # Inserts
    for first, last, gender in to_insert:
        player_id = str(uuid.uuid4())
        conn.execute(
            """INSERT INTO players (id, first_name, last_name, gender, reporting_privacy, club_id, created_at, updated_at)
               VALUES (?, ?, ?, ?, 'visible', ?, ?, ?)""",
            (player_id, first, last, gender, CLUB_ID, now, now),
        )

    conn.commit()
    conn.close()

    print(f"Done! Updated {len(to_update)} players, inserted {len(to_insert)} new players.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Import Parks League players")
    parser.add_argument(
        "--xlsx",
        default="/home/jameshartt/Downloads/Parks League list 2026.xlsx",
        help="Path to the Excel file",
    )
    parser.add_argument(
        "--db",
        default="/home/jameshartt/Development/Tennis/jim-dot-tennis/tennis.db",
        help="Path to the SQLite database",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would happen without making changes",
    )
    args = parser.parse_args()

    run_import(args.xlsx, args.db, dry_run=args.dry_run)
