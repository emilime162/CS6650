"""
Load testing for the Product Search service.

Tests:
  Test 1 - Baseline:      5 users, 2 minutes
  Test 2 - Breaking Point: 20 users, 3 minutes

Run commands:
  # Test 1 - Baseline (headless)
  locust -f locustfile.py --headless -u 5 -r 1 --run-time 2m \
         --host http://<YOUR_ECS_HOST>

  # Test 2 - Breaking Point (headless)
  locust -f locustfile.py --headless -u 20 -r 2 --run-time 3m \
         --host http://<YOUR_ECS_HOST>

  # OR open the web UI (http://localhost:8089) for live graphs:
  locust -f locustfile.py --host http://<YOUR_ECS_HOST>
"""

from locust import task, between
from locust.contrib.fasthttp import FastHttpUser


# Search terms that will match various products in the catalog
SEARCH_TERMS = [
    "electronics",   # matches category — high hit rate
    "alpha",         # matches brand in name — moderate hits
    "product",       # matches all names — very high hit rate
    "home",          # matches category
    "beta",          # matches brand
    "sports",        # matches category
    "books",         # matches category
    "gamma",         # matches brand
    "clothing",      # matches category
    "delta",         # matches brand
]


class ProductSearchUser(FastHttpUser):
    """
    Simulates a user hammering the search endpoint.
    wait_time is tiny (0–0.1s) to maximize request throughput and
    expose CPU saturation quickly.
    """
    wait_time = between(0, 0.1)

    @task
    def search_products(self):
        import random
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
            f"/products/search?q={term}",
            catch_response=True
        ) as response:
            if response.status_code == 200:
                data = response.json()
                # Verify we're checking exactly 100 products
                items_checked = data.get("items_checked", 0)
                if items_checked != 100:
                    response.failure(
                        f"Expected 100 items checked, got {items_checked}"
                    )
                else:
                    response.success()
            else:
                response.failure(f"HTTP {response.status_code}")

    @task(1)
    def health_check(self):
        """Occasional health probe to verify service is still up."""
        self.client.get("/health")