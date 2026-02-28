"""
locustfile.py — CS6650 Midterm Mastery Part III
Run against broken version first, then fixed version.

Usage:
  locust -f locustfile.py --host http://localhost:8080
  Then open http://localhost:8089 to control the test.

Recommended test runs:
  Broken:  start with 10 users, ramp to 50, watch failures climb
  Fixed:   same ramp, watch failures stay near 0
"""

from locust import HttpUser, task, between
import random

QUERIES = ["alpha", "beta", "electronics", "books", "sports",
           "gamma", "delta", "home", "clothing", "toys"]

class ProductSearchUser(HttpUser):
    # Wait 0.1–0.5s between requests (aggressive load to trigger the bug)
    wait_time = between(0.1, 0.5)

    @task(4)  # weight 4: search is the hot path
    def search_products(self):
        query = random.choice(QUERIES)
        with self.client.get(
            f"/products/search?q={query}",
            name="/products/search",
            catch_response=True
        ) as response:
            if response.status_code == 200:
                data = response.json()
                # For the fixed version: log which path was taken
                ranker_status = data.get("ranker_status", "n/a")
                if ranker_status not in ("ranked", "n/a"):
                    # Degraded but NOT a failure — this is the whole point!
                    response.success()
            else:
                response.failure(f"HTTP {response.status_code}")

    @task(1)  # weight 1: health check less frequent
    def health_check(self):
        self.client.get("/health", name="/health")