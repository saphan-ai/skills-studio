#!/bin/sh
# Prints each distinct value of column $2 of the comma-separated file $1, with its count.
cut -d, -f"$2" "$1" | sort | uniq -c | sort -rn
