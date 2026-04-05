#!/usr/bin/env python3
"""
Generate report graphs from load test CSV files.

Usage:
    python3 loadtest/plot.py --files results_w5r1_1w99r.csv results_w5r1_10w90r.csv ... \
                             --labels "W5R1 1%w" "W5R1 10%w" ...

Each CSV has columns: op, key, latency_ms, stale, version
"""

import argparse
import csv
import os
import sys
from collections import defaultdict

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np


def load_csv(path):
    reads, writes = [], []
    with open(path) as f:
        reader = csv.DictReader(f)
        for row in reader:
            lat = float(row["latency_ms"])
            if row["op"] == "read":
                reads.append(lat)
            else:
                writes.append(lat)
    return reads, writes


def cdf(data):
    """Return (sorted_values, cumulative_probability) for a CDF."""
    if not data:
        return [], []
    s = sorted(data)
    n = len(s)
    y = [(i + 1) / n for i in range(n)]
    return s, y


def plot_latency_cdfs(file_label_pairs, out_dir):
    """
    One figure with two subplots (reads / writes).
    Each series = one load-test run.
    """
    fig, (ax_r, ax_w) = plt.subplots(1, 2, figsize=(14, 5))
    fig.suptitle("Latency CDF – all configurations", fontsize=13)

    for path, label in file_label_pairs:
        reads, writes = load_csv(path)
        rx, ry = cdf(reads)
        wx, wy = cdf(writes)
        if rx:
            ax_r.plot(rx, ry, label=label)
        if wx:
            ax_w.plot(wx, wy, label=label)

    for ax, title in [(ax_r, "Read latency"), (ax_w, "Write latency")]:
        ax.set_xlabel("Latency (ms)")
        ax.set_ylabel("Cumulative probability")
        ax.set_title(title)
        ax.legend(fontsize=7)
        ax.grid(True, alpha=0.3)

    out = os.path.join(out_dir, "latency_cdf.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


def plot_stale_reads(file_label_pairs, out_dir):
    """Bar chart: stale read percentage per configuration."""
    labels, pcts = [], []
    for path, label in file_label_pairs:
        total_reads = 0
        stale = 0
        with open(path) as f:
            reader = csv.DictReader(f)
            for row in reader:
                if row["op"] == "read":
                    total_reads += 1
                    if row["stale"] == "true":
                        stale += 1
        pct = 100 * stale / total_reads if total_reads > 0 else 0
        labels.append(label)
        pcts.append(pct)

    fig, ax = plt.subplots(figsize=(10, 5))
    bars = ax.bar(labels, pcts, color="steelblue")
    ax.set_ylabel("Stale reads (%)")
    ax.set_title("Stale read rate per configuration")
    ax.set_ylim(0, max(pcts) * 1.2 + 1)
    for bar, pct in zip(bars, pcts):
        ax.text(bar.get_x() + bar.get_width() / 2, bar.get_height() + 0.3,
                f"{pct:.1f}%", ha="center", va="bottom", fontsize=8)
    plt.xticks(rotation=20, ha="right", fontsize=8)

    out = os.path.join(out_dir, "stale_reads.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


def plot_avg_latency_by_ratio(file_label_pairs, out_dir):
    """
    Grouped bar chart: average read / write latency for each config.
    """
    configs = []
    avg_reads = []
    avg_writes = []

    for path, label in file_label_pairs:
        reads, writes = load_csv(path)
        avg_reads.append(np.mean(reads) if reads else 0)
        avg_writes.append(np.mean(writes) if writes else 0)
        configs.append(label)

    x = np.arange(len(configs))
    width = 0.35

    fig, ax = plt.subplots(figsize=(12, 5))
    ax.bar(x - width / 2, avg_reads, width, label="Reads", color="steelblue")
    ax.bar(x + width / 2, avg_writes, width, label="Writes", color="coral")
    ax.set_xticks(x)
    ax.set_xticklabels(configs, rotation=20, ha="right", fontsize=8)
    ax.set_ylabel("Average latency (ms)")
    ax.set_title("Average read vs write latency per configuration")
    ax.legend()
    ax.grid(axis="y", alpha=0.3)

    out = os.path.join(out_dir, "avg_latency.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--files", nargs="+", required=True, help="CSV result files")
    parser.add_argument("--labels", nargs="+", required=True, help="Labels matching --files")
    parser.add_argument("--out-dir", default="report_graphs", help="Output directory")
    args = parser.parse_args()

    if len(args.files) != len(args.labels):
        print("ERROR: --files and --labels must have the same number of entries")
        sys.exit(1)

    os.makedirs(args.out_dir, exist_ok=True)
    pairs = list(zip(args.files, args.labels))

    plot_latency_cdfs(pairs, args.out_dir)
    plot_stale_reads(pairs, args.out_dir)
    plot_avg_latency_by_ratio(pairs, args.out_dir)

    print("Done. All graphs saved to", args.out_dir)


if __name__ == "__main__":
    main()
