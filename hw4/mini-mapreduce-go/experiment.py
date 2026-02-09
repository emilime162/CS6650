import time
import csv
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed

SPLITTER_IP = "44.247.126.63"
MAPPER_IPS_ALL = ["34.219.245.224", "18.246.237.72", "35.86.201.59"]
REDUCER_IP = "35.88.127.168"

BUCKET = "cs6650-mini-mapreduce"
INPUT_KEY = "book.txt"

TIMEOUT = 120


def post_json(url, payload):
    t0 = time.time()
    r = requests.post(url, json=payload, timeout=TIMEOUT)
    dt = time.time() - t0
    r.raise_for_status()
    return r.json(), dt


def run_once(num_chunks: int, mapper_count: int):
    mappers = MAPPER_IPS_ALL[:mapper_count]
    timings = {}

    # --- split ---
    split_res, dt = post_json(
        f"http://{SPLITTER_IP}:8080/split",
        {"bucket": BUCKET, "key": INPUT_KEY, "num_chunks": num_chunks},
    )
    timings["split"] = dt
    run_id = split_res["run_id"]
    chunk_urls = split_res["chunk_urls"]

    # --- map (CONCURRENT) ---
    map_urls = [None] * len(chunk_urls)
    map_times = [0.0] * len(chunk_urls)

    t_map0 = time.time()

    # Concurrency: don't exceed #chunks; also keep it reasonable vs mapper_count
    max_workers = min(len(chunk_urls), max(1, mapper_count))

    with ThreadPoolExecutor(max_workers=max_workers) as ex:
        futs = []
        for i, chunk_url in enumerate(chunk_urls):
            ip = mappers[i % len(mappers)]  # round-robin assignment
            fut = ex.submit(post_json, f"http://{ip}:8080/map", {"chunk_url": chunk_url})
            futs.append((i, fut))

        for i, fut in futs:
            res, dt = fut.result()
            map_urls[i] = res["map_url"]
            map_times[i] = dt

    timings["map_wall"] = time.time() - t_map0
    timings["map_serial_sum"] = sum(map_times)
    timings["map_max"] = max(map_times) if map_times else 0.0

    # --- reduce ---
    reduce_res, dt = post_json(
        f"http://{REDUCER_IP}:8080/reduce",
        {"map_urls": map_urls},
    )
    timings["reduce"] = dt

    # serial baseline vs parallel estimate
    timings["end_to_end_serial"] = timings["split"] + timings["map_serial_sum"] + timings["reduce"]
    timings["end_to_end_parallel_est"] = timings["split"] + timings["map_wall"] + timings["reduce"]

    return {
        "run_id": run_id,
        "num_chunks": num_chunks,
        "mapper_count": mapper_count,
        "result_url": reduce_res["result_url"],
        **timings,
    }


def main():
    chunk_list = [1, 2, 3, 6, 9]
    mapper_counts = [1, 2, 3]
    repeats = 5

    rows = []
    for mc in mapper_counts:
        for nc in chunk_list:
            for r in range(repeats):
                out = run_once(nc, mc)

                # Real speedup uses map_wall
                speedup_real = out["map_serial_sum"] / out["map_wall"] if out["map_wall"] > 0 else 0.0

                print(
                    f"mc={mc} nc={nc} rep={r+1}/{repeats} "
                    f"map_wall={out['map_wall']:.3f}s "
                    f"speedup_real={speedup_real:.2f}x "
                    f"e2e_parallel={out['end_to_end_parallel_est']:.3f}s"
                )
                rows.append(out)

    with open("results_concurrent.csv", "w", newline="") as f:
        w = csv.DictWriter(
            f,
            fieldnames=[
                "run_id", "num_chunks", "mapper_count",
                "split", "map_serial_sum", "map_max", "map_wall",
                "reduce", "end_to_end_serial", "end_to_end_parallel_est",
                "result_url",
            ],
        )
        w.writeheader()
        w.writerows(rows)

    print("Wrote results_concurrent.csv")


if __name__ == "__main__":
    main()
