# Notes on quoted fields

The script cuts at every comma. A field that holds a comma inside quotes is
split in two, so the counts for such a file are wrong. Say so to the user and
suggest a tool that reads quoted fields.
