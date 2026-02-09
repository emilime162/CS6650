import requests
import time
import matplotlib.pyplot as plt
import numpy as np
from datetime import datetime

def load_test(url, duration_seconds=30, timeout=10, sleep_seconds=0.0):
    """
    Sends sequential GET requests for a fixed duration.
    Records response time (ms), status codes, and errors.
    """
    response_times = []
    status_codes = []
    errors = 0

    start_time = time.time()
    end_time = start_time + duration_seconds

    print(f"Starting load test for {duration_seconds} seconds...")
    print(f"Target URL: {url}")

    while time.time() < end_time:
        try:
            t0 = time.time()
            response = requests.get(url, timeout=timeout)
            t1 = time.time()

            rt_ms = (t1 - t0) * 1000.0
            response_times.append(rt_ms)
            status_codes.append(response.status_code)

            if response.status_code == 200:
                print(f"Request {len(response_times)}: {rt_ms:.2f}ms")
            else:
                print(f"Request {len(response_times)}: {rt_ms:.2f}ms (status {response.status_code})")

        except requests.exceptions.RequestException as e:
            errors += 1
            print(f"Request failed: {e}")

        if sleep_seconds > 0:
            time.sleep(sleep_seconds)

    return np.array(response_times), np.array(status_codes), errors


def print_stats(response_times, status_codes, errors):
    total_attempts = len(response_times) + errors
    ok_times = response_times[status_codes == 200]
    non200 = int(np.sum(status_codes != 200))

    print("\nStatistics:")
    print(f"Total attempts: {total_attempts}")
    print(f"200 OK:         {len(ok_times)}")
    print(f"Non-200:        {non200}")
    print(f"Exceptions:     {errors}")

    if len(ok_times) == 0:
        print("No successful (200 OK) requests to compute latency stats.")
        return

    print("\nLatency stats for 200 OK only:")
    print(f"Average:        {np.mean(ok_times):.2f}ms")
    print(f"Median:         {np.median(ok_times):.2f}ms")
    print(f"95th percentile:{np.percentile(ok_times, 95):.2f}ms")
    print(f"99th percentile:{np.percentile(ok_times, 99):.2f}ms")
    print(f"Max:            {np.max(ok_times):.2f}ms")


def plot_results(response_times, status_codes):
    ok_times = response_times[status_codes == 200]

    run_tag = datetime.now().strftime("%Y%m%d_%H%M%S")

    plt.figure(figsize=(12, 8))

    # Histogram (200 OK only)
    plt.subplot(2, 1, 1)
    plt.hist(ok_times, bins=50, alpha=0.7)
    plt.xlabel("Response Time (ms)")
    plt.ylabel("Frequency")
    plt.title("Distribution of Response Times (200 OK)")

    # Scatter plot over time (use request index)
    plt.subplot(2, 1, 2)
    plt.scatter(range(len(ok_times)), ok_times, alpha=0.6, s=14)
    plt.xlabel("Request Number (200 OK)")
    plt.ylabel("Response Time (ms)")
    plt.title("Response Times Over Time (200 OK)")

    plt.tight_layout()

    # Save figures for submission
    out_file = f"load_test_{run_tag}.png"
    plt.savefig(out_file, dpi=160)
    plt.show()

    print(f"\nSaved plot image: {out_file}")


if __name__ == "__main__":
    EC2_PUBLIC_IP = "34.218.45.197"  
    EC2_URL = f"http://{EC2_PUBLIC_IP}:8080/albums"

    # Run the test
    response_times, status_codes, errors = load_test(
        EC2_URL,
        duration_seconds=30,   
        timeout=10,
        sleep_seconds=0.0     
    )

    # Print stats + plot
    print_stats(response_times, status_codes, errors)
    plot_results(response_times, status_codes)
