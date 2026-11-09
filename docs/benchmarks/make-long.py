#!/usr/bin/env python3
"""Make 80k invented, 2048-byte ASCII lines in the comparison frame format."""
import argparse
import pathlib
import struct

p = argparse.ArgumentParser(description=__doc__)
p.add_argument("output", type=pathlib.Path)
args = p.parse_args()
records = []
for i in range(80000):
    accepted = i % 7 != 0
    head = f"component-{i % 31} {'accepted' if accepted else 'rejected'} user-{i % 997} "
    tail = f" status {'ok' if accepted else 'error'}"
    line = (head + "x" * (2048 - len(head) - len(tail)) + tail).encode("ascii")
    assert len(line) == 2048
    records.append((i % 3, line))
records.sort()
with args.output.open("xb") as f:
    for partition, line in records:
        f.write(struct.pack("<II", partition, len(line)))
        f.write(line)
