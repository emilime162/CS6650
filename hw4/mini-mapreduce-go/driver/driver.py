import time
import requests

SPLITTER_IP = "44.247.126.63"

MAPPER_IPS = [
    "34.219.245.224",
    "18.246.237.72",
    "35.86.201.59",
]

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


def main():
    timings = {}

    print("=== STEP 1: SPLIT ===")
    split_url = f"http://{SPLITTER_IP}:8080/split"
    split_res, dt = post_json(
        split_url,
        {
            "bucket": BUCKET,
            "key": INPUT_KEY,
            "num_chunks": 3,
        },
    )
    timings["split"] = dt

    run_id = split_res["run_id"]
    chunk_urls = split_res["chunk_urls"]

    print("run_id:", run_id)
    print("chunk_urls:", chunk_urls)

    print("\n=== STEP 2: MAP ===")
    map_urls = []
    map_times = []

    for ip, chunk_url in zip(MAPPER_IPS, chunk_urls):
        map_url = f"http://{ip}:8080/map"
        res, dt = post_json(map_url, {"chunk_url": chunk_url})
        map_urls.append(res["map_url"])
        map_times.append(dt)

    timings["map_serial_sum"] = sum(map_times)
    timings["map_max"] = max(map_times)

    print("map_urls:", map_urls)
    print("map_times:", map_times)

    print("\n=== STEP 3: REDUCE ===")
    reduce_url = f"http://{REDUCER_IP}:8080/reduce"
    reduce_res, dt = post_json(reduce_url, {"map_urls": map_urls})
    timings["reduce"] = dt

    result_url = reduce_res["result_url"]
    print("result_url:", result_url)

    timings["end_to_end_serial"] = (
        timings["split"]
        + timings["map_serial_sum"]
        + timings["reduce"]
    )
 
    print("\n=== TIMINGS (seconds) ===")
    for k, v in timings.items():
        print(f"{k}: {v:.3f}")


if __name__ == "__main__":
    main()
