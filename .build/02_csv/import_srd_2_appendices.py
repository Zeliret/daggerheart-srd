#!/usr/bin/env python3
"""Import SRD 2.0 appendix tables using their PDF column geometry.

The tagged PDF text interleaves adjacent columns. pdfplumber preserves the
table cell positions, allowing this importer to populate the existing CSV
schemas without changing consumer-facing field names.
"""
from __future__ import annotations

import csv
import re
from collections import defaultdict
from pathlib import Path

import pdfplumber

ROOT = Path(__file__).resolve().parents[2]
PDF = ROOT / ".build/01_pdf/DH-SRD-2.0-2026-08-25.pdf"
CSV = ROOT / ".build/02_csv"


def compact(text: str) -> str:
    return re.sub(r"\s+", " ", text or "").strip()


def write_rows(name: str, header: list[str], rows: list[dict[str, str]]) -> None:
    with (CSV / name).open("w", newline="") as fh:
        writer = csv.DictWriter(fh, fieldnames=header)
        writer.writeheader()
        writer.writerows(rows)


def table_lines(page):
    buckets = defaultdict(list)
    for word in page.extract_words(use_text_flow=True):
        buckets[round(word["top"] / 4) * 4].append(word)
    return [sorted(words, key=lambda w: w["x0"]) for _, words in sorted(buckets.items())]


def loot_rows(pdf, page_range: range) -> list[dict[str, str]]:
    # Item and consumable pages use x=84 for rolls, x=110 for names, x>=200
    # for descriptions. Continuation lines retain their respective columns.
    records = []
    active = {}
    for page_no in page_range:
        page = pdf.pages[page_no]
        # Items (pages 75–79) are one table per page; consumables use paired
        # tables. Page text also contains ordinary numbers at x≈318, so do not
        # infer table count from arbitrary numeric words.
        two_columns = page_range.start != 74
        for words in table_lines(pdf.pages[page_no]):
            # A page may contain two independent roll/name/description tables.
            bases = [70, 318] if two_columns else [70]
            for base in bases:
                limit = base + 245 if two_columns else 612
                roll_words = [w["text"] for w in words if base - 18 <= w["x0"] < base + 22]
                name_words = [w["text"] for w in words if base + 22 <= w["x0"] < base + 90]
                desc_words = [w["text"] for w in words if base + 90 <= w["x0"] < limit]
                roll = compact(" ".join(roll_words))
                key = base
                if re.fullmatch(r"\d{1,2}", roll):
                    if key in active: records.append(active[key])
                    active[key] = {"Roll": roll.zfill(2), "Name": compact(" ".join(name_words)), "Description": compact(" ".join(desc_words))}
                elif key in active:
                    if name_words:
                        active[key]["Name"] = compact(active[key]["Name"] + " " + " ".join(name_words))
                    if desc_words:
                        active[key]["Description"] = compact(active[key]["Description"] + " " + " ".join(desc_words))
        records.extend(active.values())
        active.clear()
    for record in records:
        record["Name"] = re.sub(r"\s*Daggerheart SRD.*$", "", record["Name"]).strip()
        record["Name"] = re.sub(r"\s+SRD$", "", record["Name"]).strip()
        record["Name"] = re.sub(r"\s+Items following table includes Loot.*$", "", record["Name"]).strip()
        record["Description"] = re.sub(r"\s*Daggerheart SRD.*$", "", record["Description"]).strip()
    return [r for r in records if r["Name"] and r["Description"]]


def adversaries(pdf) -> list[dict[str, str]]:
    records = []
    tier = 0
    # PDF pages 97–154 are the adversary appendix; each statblock occupies one
    # half-page, so crop before text extraction to prevent column interleaving.
    for page_no in range(96, 154):
        page = pdf.pages[page_no]
        for left, right in ((0, page.width / 2), (page.width / 2, page.width)):
            text = page.crop((left, 0, right, page.height)).extract_text() or ""
            tier_match = re.search(r"TIER ([1-4]) ADVERSARIES", text)
            if tier_match: tier = int(tier_match.group(1))
            starts = list(re.finditer(r"(?m)^([A-Z][A-Z ’'\-]+)\nTier [^\n]*? (Solo|Leader|Bruiser|Skulk|Standard|Minion|Ranged)\n", text))
            for i, match in enumerate(starts):
                block = text[match.start(): starts[i + 1].start() if i + 1 < len(starts) else len(text)]
                stat = re.search(r"Diffi\s*culty:\s*([^|]+)\|\s*Thresholds:\s*([^|]+)\|\s*HP:\s*([^|]+)\|\s*Stress:\s*([^\n]+)", block)
                attack = re.search(r"ATK:\s*([^|]+)\|\s*([^:]+):\s*([^|]+)\|\s*([^\n]+)", block)
                motive = re.search(r"Motives\s*& Tactics:\s*(.+)", block)
                experience = re.search(r"Experience:\s*(.+)", block)
                before_motive = block[:motive.start()] if motive else block
                description = "\n".join(before_motive.splitlines()[2:])
                feature_text = block.split("FEATURES", 1)[1] if "FEATURES" in block else ""
                features = list(re.finditer(r"(?m)^(.+?) - (?:Passive|Action|Reaction):\s*", feature_text))
                row = {"Name": compact(match.group(1).title()), "Tier": str(tier), "Type": match.group(2), "Description": compact(description), "Motives and Tactics": compact(motive.group(1)) if motive else ""}
                if stat: row.update(dict(zip(("Difficulty", "Thresholds", "HP", "Stress"), map(compact, stat.groups()))))
                if attack: row.update({"ATK": compact(attack.group(1)), "Attack": compact(attack.group(2)), "Range": compact(attack.group(3)), "Damage": compact(attack.group(4))})
                if experience: row["Experience"] = compact(experience.group(1))
                for n, feature in enumerate(features[:7], 1):
                    end = features[n].start() if n < len(features) else len(feature_text)
                    row[f"Feature {n} Name"] = compact(feature.group(1))
                    row[f"Feature {n} Text"] = compact(feature_text[feature.end():end])
                records.append(row)
    return records


def main() -> None:
    with pdfplumber.open(PDF) as pdf:
        items = loot_rows(pdf, range(74, 79))
        consumables = loot_rows(pdf, range(79, 86))
        foes = adversaries(pdf)
    if len(items) < 100 or len(consumables) < 100 or len(foes) < 150:
        raise SystemExit(f"unexpected extraction counts: items={len(items)}, consumables={len(consumables)}, adversaries={len(foes)}")
    for filename, rows in (("items.csv", items), ("consumables.csv", consumables), ("adversaries.csv", foes)):
        with (CSV / filename).open(newline="") as fh: header = next(csv.reader(fh))
        write_rows(filename, header, rows)
    print(f"imported {len(items)} items, {len(consumables)} consumables, and {len(foes)} adversaries")


if __name__ == "__main__":
    main()
