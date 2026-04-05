#!/usr/bin/env python3
"""
Generate report graphs from load test CSV files.

Produces one figure per write ratio (1%, 10%, 50%, 90%), each containing:
  - Read latency CDF  (all 4 configs at that ratio)
  - Write latency CDF (all 4 configs at that ratio)
  - Read-write interval distribution (all 4 configs at that ratio)

Read and write latency are on SEPARATE subplots with their own y-axes so
that 2ms reads and 810ms writes are both visible.

Also produces summary graphs across all configurations:
  - stale_reads.png  – stale read % bar chart
  - avg_latency_reads.png  – average read latency only
  - avg_latency_writes.png – average write latency only

Usage:
    python3 loadtest/plot.py \\
        --files  results_lf_w5r1_1w.csv  results_lf_w1r5_1w.csv  \\
                 results_lf_quorum_1w.csv results_leaderless_1w.csv \\
                 results_lf_w5r1_10w.csv results_lf_w1r5_10w.csv  \\
                 results_lf_quorum_10w.csv results_leaderless_10w.csv \\
                 results_lf_w5r1_50w.csv results_lf_w1r5_50w.csv  \\
                 results_lf_quorum_50w.csv results_leaderless_50w.csv \\
                 results_lf_w5r1_90w.csv results_lf_w1r5_90w.csv  \\
                 results_lf_quorum_90w.csv results_leaderless_90w.csv \\
        --labels "W5R1" "W1R5" "Quorum" "Leaderless" \\
        --ratios "1% writes" "10% writes" "50% writes" "90% writes" \\
        --out-dir report_graphs

IMPORTANT:
  --files  must be ordered: all configs for ratio1, then all configs for ratio2, etc.
           Within each ratio the order must match --labels.
  --labels are the 4 config names (repeated for each ratio internally).
  --ratios are the 4 ratio names.
"""

import argparse
import csv
import os
import sys

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np


# ── Data loading ──────────────────────────────────────────────────────────────

def load_csv(path):
    """
    Returns:
        reads     – list of read latency floats (ms)
        writes    – list of write latency floats (ms)
        intervals – list of floats: ms between last write on a key and the
                    next read on the same key (sorted by timestamp)
        stale_pct – percentage of reads that were stale
    """
    rows = []
    with open(path) as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows.append(row)

    rows.sort(key=lambda r: int(r["timestamp_ms"]))

    reads, writes = [], []
    last_write_ts = {}
    intervals = []
    total_reads = stale_reads = 0

    for row in rows:
        lat = float(row["latency_ms"])
        ts  = int(row["timestamp_ms"])
        key = row["key"]

        if row["op"] == "write":
            writes.append(lat)
            last_write_ts[key] = ts
        else:
            reads.append(lat)
            total_reads += 1
            if row["stale"] == "true":
                stale_reads += 1
            if key in last_write_ts:
                interval = ts - last_write_ts[key]
                if interval >= 0:
                    intervals.append(interval)

    stale_pct = 100 * stale_reads / total_reads if total_reads else 0
    return reads, writes, intervals, stale_pct


# ── CDF helper ────────────────────────────────────────────────────────────────

def cdf(data):
    if not data:
        return [], []
    s = sorted(data)
    y = [(i + 1) / len(s) for i in range(len(s))]
    return s, y


# ── Colors ────────────────────────────────────────────────────────────────────

COLORS = ["#2196F3", "#F44336", "#4CAF50", "#FF9800"]  # blue, red, green, orange


# ── Per-ratio figure ──────────────────────────────────────────────────────────

def plot_ratio(ratio_label, files_for_ratio, config_labels, out_dir):
    """
    One figure with 3 rows × 1 column:
      Row 1: Read latency CDF
      Row 2: Write latency CDF
      Row 3: Read-write interval histograms (one subplot per config)
    """
    n = len(config_labels)
    fig = plt.figure(figsize=(12, 14))
    fig.suptitle(f"Latency Results – {ratio_label}", fontsize=15, fontweight="bold", y=0.98)

    # ── Row 1: Read latency CDF ───────────────────────────────────────────────
    ax_read = fig.add_subplot(3, 1, 1)
    for i, (path, label) in enumerate(zip(files_for_ratio, config_labels)):
        reads, _, _, _ = load_csv(path)
        rx, ry = cdf(reads)
        if rx:
            ax_read.plot(rx, ry, label=label, color=COLORS[i], linewidth=2)

    ax_read.set_title("Read Latency CDF", fontsize=12, fontweight="bold")
    ax_read.set_xlabel("Latency (ms)")
    ax_read.set_ylabel("Cumulative probability")
    ax_read.legend(fontsize=10)
    ax_read.grid(True, alpha=0.3)
    # Zoom x-axis to show read latencies clearly (max 200ms)
    ax_read.set_xlim(left=0)
    all_reads = []
    for path in files_for_ratio:
        r, _, _, _ = load_csv(path)
        all_reads.extend(r)
    if all_reads:
        p99 = np.percentile(all_reads, 99)
        ax_read.set_xlim(right=max(p99 * 1.1, 5))

    # ── Row 2: Write latency CDF ──────────────────────────────────────────────
    ax_write = fig.add_subplot(3, 1, 2)
    for i, (path, label) in enumerate(zip(files_for_ratio, config_labels)):
        _, writes, _, _ = load_csv(path)
        wx, wy = cdf(writes)
        if wx:
            ax_write.plot(wx, wy, label=label, color=COLORS[i], linewidth=2)

    ax_write.set_title("Write Latency CDF", fontsize=12, fontweight="bold")
    ax_write.set_xlabel("Latency (ms)")
    ax_write.set_ylabel("Cumulative probability")
    ax_write.legend(fontsize=10)
    ax_write.grid(True, alpha=0.3)
    ax_write.set_xlim(left=0)

    # ── Row 3: Read-write interval (4 subplots side by side) ──────────────────
    for i, (path, label) in enumerate(zip(files_for_ratio, config_labels)):
        ax = fig.add_subplot(3, n, 2 * n + i + 1)
        _, _, intervals, _ = load_csv(path)

        if not intervals:
            ax.text(0.5, 0.5, "no intervals\n(too few writes)",
                    ha="center", va="center", transform=ax.transAxes,
                    fontsize=9, color="gray")
        else:
            arr = np.array(intervals)
            p50 = np.percentile(arr, 50)
            p95 = np.percentile(arr, 95)
            ax.hist(arr, bins=30, color=COLORS[i], alpha=0.75, edgecolor="white")
            ax.axvline(p50, color="black",  linestyle="--", linewidth=1.2,
                       label=f"p50={p50:.0f}ms")
            ax.axvline(p95, color="red",    linestyle=":",  linewidth=1.2,
                       label=f"p95={p95:.0f}ms")
            ax.legend(fontsize=7)

        ax.set_title(f"RW Interval – {label}", fontsize=9, fontweight="bold")
        ax.set_xlabel("Interval (ms)", fontsize=8)
        ax.set_ylabel("Count", fontsize=8)
        ax.grid(True, alpha=0.3)

    fig.tight_layout(rect=[0, 0, 1, 0.97])
    ratio_slug = ratio_label.replace("%", "pct").replace(" ", "_")
    out = os.path.join(out_dir, f"ratio_{ratio_slug}.png")
    fig.savefig(out, dpi=150, bbox_inches="tight")
    print(f"Saved {out}")
    plt.close(fig)


# ── Summary: stale reads ──────────────────────────────────────────────────────

def plot_stale_reads(all_files, all_labels, out_dir):
    pcts = []
    for path in all_files:
        _, _, _, sp = load_csv(path)
        pcts.append(sp)

    fig, ax = plt.subplots(figsize=(14, 5))
    bars = ax.bar(all_labels, pcts, color="#2196F3", alpha=0.85)
    ax.set_ylabel("Stale reads (%)", fontsize=12)
    ax.set_title("Stale Read Rate per Configuration", fontsize=13, fontweight="bold")
    ax.set_ylim(0, max(pcts) * 1.3 + 1)
    for bar, pct in zip(bars, pcts):
        ax.text(bar.get_x() + bar.get_width() / 2, bar.get_height() + 0.4,
                f"{pct:.1f}%", ha="center", va="bottom", fontsize=7.5)
    plt.xticks(rotation=35, ha="right", fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    out = os.path.join(out_dir, "stale_reads.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


# ── Summary: avg latency (reads and writes on separate charts) ────────────────

def plot_avg_latency(all_files, all_labels, out_dir):
    avg_r, avg_w = [], []
    for path in all_files:
        reads, writes, _, _ = load_csv(path)
        avg_r.append(np.mean(reads)  if reads  else 0)
        avg_w.append(np.mean(writes) if writes else 0)

    # Reads
    fig, ax = plt.subplots(figsize=(14, 5))
    ax.bar(all_labels, avg_r, color="#2196F3", alpha=0.85)
    ax.set_ylabel("Average latency (ms)", fontsize=12)
    ax.set_title("Average Read Latency per Configuration", fontsize=13, fontweight="bold")
    plt.xticks(rotation=35, ha="right", fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    out = os.path.join(out_dir, "avg_latency_reads.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)

    # Writes
    fig, ax = plt.subplots(figsize=(14, 5))
    ax.bar(all_labels, avg_w, color="#F44336", alpha=0.85)
    ax.set_ylabel("Average latency (ms)", fontsize=12)
    ax.set_title("Average Write Latency per Configuration", fontsize=13, fontweight="bold")
    plt.xticks(rotation=35, ha="right", fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    out = os.path.join(out_dir, "avg_latency_writes.png")
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    print(f"Saved {out}")
    plt.close(fig)


# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--files",  nargs="+", required=True,
                        help="CSV files ordered: all configs for ratio1, then ratio2, ...")
    parser.add_argument("--labels", nargs="+", required=True,
                        help="Config labels e.g. W5R1 W1R5 Quorum Leaderless")
    parser.add_argument("--ratios", nargs="+", required=True,
                        help="Ratio labels e.g. '1%% writes' '10%% writes' ...")
    parser.add_argument("--out-dir", default="report_graphs")
    args = parser.parse_args()

    n_configs = len(args.labels)
    n_ratios  = len(args.ratios)

    if len(args.files) != n_configs * n_ratios:
        print(f"ERROR: expected {n_configs} configs × {n_ratios} ratios = "
              f"{n_configs * n_ratios} files, got {len(args.files)}")
        sys.exit(1)

    os.makedirs(args.out_dir, exist_ok=True)

    # ── Per-ratio figures ─────────────────────────────────────────────────────
    for r_idx, ratio_label in enumerate(args.ratios):
        files_for_ratio = args.files[r_idx * n_configs : (r_idx + 1) * n_configs]
        plot_ratio(ratio_label, files_for_ratio, args.labels, args.out_dir)

    # ── Summary figures ───────────────────────────────────────────────────────
    # Build flat label list: "W5R1 1%w", "W1R5 1%w", ..., "W5R1 10%w", ...
    flat_labels = []
    for ratio in args.ratios:
        for label in args.labels:
            flat_labels.append(f"{label}\n{ratio}")

    plot_stale_reads(args.files, flat_labels, args.out_dir)
    plot_avg_latency(args.files, flat_labels, args.out_dir)

    print(f"\nAll graphs saved to: {args.out_dir}/")
    print("Per-ratio files: ratio_1pct_writes.png, ratio_10pct_writes.png, ...")
    print("Summary files:   stale_reads.png, avg_latency_reads.png, avg_latency_writes.png")


if __name__ == "__main__":
    main()