#!/usr/bin/env python3
"""Check project text files without printing environment values or other content."""

import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
EXCLUDED_DIRS = {
    ".git", "node_modules", "vendor", "data", "dist", ".idea", ".vscode",
    "backups", "releases", "secrets", "__pycache__",
}
BINARY_SUFFIXES = {
    ".exe", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff",
    ".woff2", ".ttf", ".mp4", ".mp3", ".pdf", ".zip", ".gz", ".db",
    ".docx", ".sqlite", ".sqlite3", ".pem", ".key", ".out", ".test", ".log",
}
MOJIBAKE = re.compile(
    r"\u951f\u65a4\u62f7|\u70eb\u70eb|\u5c6f\u5c6f|\u923a\u6128\u6672"
    r"|\u00e2\u20ac|\u00f0\u0178|\u00ef\u00bb\u00bf"
)
INVALID_TEXT_CHARACTERS = re.compile(r"[\ufffd\ue000-\uf8ff\u0080-\u009f]")


def check(root: Path) -> tuple[int, list[str]]:
    checked = 0
    issues = []
    for directory, dirs, files in os.walk(root):
        dirs[:] = sorted(
            d for d in dirs if d not in EXCLUDED_DIRS and not d.startswith("data.bak")
        )
        for name in sorted(files):
            path = Path(directory) / name
            if (
                path.suffix.lower() in BINARY_SUFFIXES
                or ".db-" in name
                or any(part in name.lower() for part in ("credentials", "secrets"))
            ):
                continue
            raw = path.read_bytes()
            is_utf16 = raw.startswith((b"\xff\xfe", b"\xfe\xff"))
            if b"\0" in raw and not is_utf16:
                continue
            checked += 1
            relative = path.relative_to(root).as_posix()
            try:
                content = raw.decode("utf-8-sig")
            except UnicodeDecodeError as error:
                issues.append(f"{relative}: invalid UTF-8 at byte {error.start}")
                continue
            invalid_lines = {
                content.count("\n", 0, match.start()) + 1
                for match in INVALID_TEXT_CHARACTERS.finditer(content)
            }
            for line_number in sorted(invalid_lines):
                issues.append(f"{relative}:{line_number}: suspicious encoding characters")
            for line_number, line in enumerate(content.split("\n"), 1):
                if line_number not in invalid_lines and MOJIBAKE.search(line):
                    issues.append(f"{relative}:{line_number}: suspicious encoding characters")
                if path.name == ".env" and line.lstrip().startswith("#"):
                    # Catch assignments swallowed by a corrupted comment/newline.
                    if re.search(r"(?<![A-Za-z_])[A-Z_][A-Z0-9_]*\s*=", line) and not re.match(
                        r"\s*#\s*(?:export\s+)?[A-Z_][A-Z0-9_]*\s*=", line
                    ):
                        issues.append(f"{relative}:{line_number}: assignment embedded in comment")
    return checked, issues


def main() -> int:
    sys.stdout.reconfigure(encoding="utf-8")
    checked, issues = check(ROOT)
    print(f"Checked {checked} project text files.")
    if issues:
        for issue in issues:
            print(issue)
        print(f"Found {len(issues)} encoding issues.")
        return 1
    print("No invalid UTF-8 or known mojibake found.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
