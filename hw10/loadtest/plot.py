#!/usr/bin/env python3
"""
Generate the three required report graphs from load test CSV files.

Graph 1: CDF of read latency per configuration
Graph 2: CDF of write latency per configuration
Graph 3: Distribution of time interval between a write and the next read
         on the same key (per configuration)

CSV columns: op, key, latency_ms, stale, version, timestamp_ms

Usage:
    python3 loadtest/plot.py \
        --files results_lf_w5r1_10w.csv results_lf_w1r5_10w.csv ... \
        --labels "W5R1 10%w" "W1R5 10%w" ... \
        --out-dir report_graphs
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


# ── Data loading ──────────────────────────────────────────────────────────────

def load_csv(path):
    """
    Returns:
        reads      – list of read latency floats (ms)
        writes     – list of write latency floats (ms)
        intervals  – list of floats: ms between last write on a key and the
                     next read on that same key. Only pairs where a write
                     preceded the read are included.
    """
    rows = []
    with open(path) as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows.append(row)

    reads, writes = [], []
    # key → timestamp_ms of most recent write
    last_write_ts = {}
    intervals = []

    # Sort by timestamp so we process events in time order.
    rows.sort(key=lambda r: int(r["timestamp_ms"]))

    for row in rows:
        lat = float(row["latency_ms"])
        ts  = int(row["timestamp_ms"])
        key = row["key"]

        if row["op"] == "write":
            writes.append(lat)
            last_write_ts[key] = ts
        else:
            reads.append(lat)
            if key in last_write_ts:
                interval = ts - last_write_ts[key]
                if interval >= 0:
                    intervals.append(interval)

    return reads, writes, intervals


# ── Graph helpers ─────────────────────────────────────────────────────────────

def cdf(data):
    if not data:
        return [], []
    s = sorted(data)
    y = [(i + 1) / len(s) for i in range(len(s))]
    return s, y


def _style_ax(ax, xlabel, ylabel, title):
    ax.set_xlabel(xlabel)
    ax.set_ylabel(ylabel)
    ax.set_title(title)
    ax.legend(fontsize=7, loc="lower right")
    ax.grid(True, alpha=0.3)


# ── Graph 1 & 2: Latency CDFs ─────────────────────────────────────────────────

def plot_latency_cdfs(pairs, out_dir):
    """
    One figure, two subplots: read latency CDF and write latency CDF.
    Each line = one configuration / ratio combination.
    """
    fig, (ax_r, ax_w) = plt.subplots(1, 2, figsize=(14, 5))
    fig.suptitle("Latency CDF across all configurations", fontsize=13)

    for path, label in pairs:
        reads, writes, _ = load_csv(path)
        rx, ry = cdf(reads)
        wx, wy = cdf(writes)
        if rx:
            ax_r.plot(rx, ry, label=label, linewidth=1.2)
        if wx:
            ax_w.plot(wx, wy, label=label, linewidth=1.2)

    _style_ax(ax_r, "Latency (ms)", "Cumulative probability", "Read latency CDF")
    _style_ax(ax_w, "Latency (ms)", "Cumulative probability", "Write latency CDF")

    out = os.path.join(out_dir, "latency_cdf.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


# ── Graph 3: Read-Write interval distribution ─────────────────────────────────

def plot_rw_intervals(pairs, out_dir):
    """
    For each configuration, plot the distribution (histogram + CDF overlay)
    of the time between a write completing and the next read on the same key.

    This shows HOW CLOSELY reads follow writes in the load test – a tight
    distribution means many reads land inside the replication window,
    explaining why stale reads occur.
    """
    # One subplot per configuration so the distributions are easy to compare.
    n = len(pairs)
    cols = min(n, 4)
    rows = (n + cols - 1) // cols
    fig, axes = plt.subplots(rows, cols, figsize=(4 * cols, 3.5 * rows), squeeze=False)
    fig.suptitle("Read-Write interval distribution\n(ms between write completion and next read on same key)", fontsize=11)

    for idx, (path, label) in enumerate(pairs):
        ax = axes[idx // cols][idx % cols]
        _, _, intervals = load_csv(path)

        if not intervals:
            ax.text(0.5, 0.5, "no intervals", ha="center", va="center", transform=ax.transAxes)
            ax.set_title(label, fontsize=8)
            continue

        arr = np.array(intervals)
        p50  = np.percentile(arr, 50)
        p95  = np.percentile(arr, 95)
        p99  = np.percentile(arr, 99)

        ax.hist(arr, bins=40, color="steelblue", alpha=0.7, edgecolor="white")
        ax.axvline(p50, color="orange",  linestyle="--", linewidth=1, label=f"p50={p50:.0f}ms")
        ax.axvline(p95, color="red",     linestyle="--", linewidth=1, label=f"p95={p95:.0f}ms")
        ax.axvline(p99, color="darkred", linestyle=":",  linewidth=1, label=f"p99={p99:.0f}ms")

        ax.set_title(label, fontsize=8)
        ax.set_xlabel("Interval (ms)", fontsize=7)
        ax.set_ylabel("Count", fontsize=7)
        ax.legend(fontsize=6)
        ax.grid(True, alpha=0.3)

    # Hide unused subplots.
    for idx in range(len(pairs), rows * cols):
        axes[idx // cols][idx % cols].set_visible(False)

    out = os.path.join(out_dir, "rw_interval.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


# ── Graph 4 (bonus): Stale read % bar chart ───────────────────────────────────

def plot_stale_reads(pairs, out_dir):
    labels, pcts = [], []
    for path, label in pairs:
        total, stale = 0, 0
        with open(path) as f:
            for row in csv.DictReader(f):
                if row["op"] == "read":
                    total += 1
                    if row["stale"] == "true":
                        stale += 1
        pcts.append(100 * stale / total if total else 0)
        labels.append(label)

    fig, ax = plt.subplots(figsize=(max(10, len(labels) * 0.7), 5))
    bars = ax.bar(labels, pcts, color="steelblue")
    ax.set_ylabel("Stale reads (%)")
    ax.set_title("Stale read rate per configuration")
    ax.set_ylim(0, max(pcts) * 1.25 + 1)
    for bar, pct in zip(bars, pcts):
        ax.text(bar.get_x() + bar.get_width() / 2, bar.get_height() + 0.3,
                f"{pct:.1f}%", ha="center", va="bottom", fontsize=7)
    plt.xticks(rotation=30, ha="right", fontsize=7)

    out = os.path.join(out_dir, "stale_reads.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


# ── Graph 5 (bonus): Average latency grouped bar ──────────────────────────────

def plot_avg_latency(pairs, out_dir):
    configs, avg_r, avg_w = [], [], []
    for path, label in pairs:
        reads, writes, _ = load_csv(path)
        avg_r.append(np.mean(reads)  if reads  else 0)
        avg_w.append(np.mean(writes) if writes else 0)
        configs.append(label)

    x = np.arange(len(configs))
    w = 0.35
    fig, ax = plt.subplots(figsize=(max(12, len(configs) * 0.8), 5))
    ax.bar(x - w / 2, avg_r, w, label="Reads",  color="steelblue")
    ax.bar(x + w / 2, avg_w, w, label="Writes", color="coral")
    ax.set_xticks(x)
    ax.set_xticklabels(configs, rotation=30, ha="right", fontsize=7)
    ax.set_ylabel("Average latency (ms)")
    ax.set_title("Average read vs write latency")
    ax.legend()
    ax.grid(axis="y", alpha=0.3)

    out = os.path.join(out_dir, "avg_latency.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--files",   nargs="+", required=True)
    parser.add_argument("--labels",  nargs="+", required=True)
    parser.add_argument("--out-dir", default="report_graphs")
    args = parser.parse_args()

    if len(args.files) != len(args.labels):
        print("ERROR: --files and --labels must have the same length")
        sys.exit(1)

    os.makedirs(args.out_dir, exist_ok=True)
    pairs = list(zip(args.files, args.labels))

    plot_latency_cdfs(pairs, args.out_dir)   # Graph 1 + 2 (reads & writes)
    plot_rw_intervals(pairs, args.out_dir)   # Graph 3 (read-write interval)
    plot_stale_reads(pairs, args.out_dir)    # Bonus: stale % bar
    plot_avg_latency(pairs, args.out_dir)    # Bonus: avg latency bar

    print(f"\nAll graphs saved to: {args.out_dir}/")


if __name__ == "__main__":
    main()