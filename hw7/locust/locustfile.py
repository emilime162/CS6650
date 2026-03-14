"""
Locust load test for HW7 Order Service.

Usage:
    # Normal load (Phase 1 test):
    locust -f locustfile.py --host=http://YOUR-ALB-DNS \
           --users=5 --spawn-rate=1 --run-time=30s --headless

    # Flash sale load (Phase 1 & 3 tests):
    locust -f locustfile.py --host=http://YOUR-ALB-DNS \
           --users=20 --spawn-rate=10 --run-time=60s --headless

Set TEST_MODE env var to switch endpoints:
    TEST_MODE=sync   → POST /orders/sync
    TEST_MODE=async  → POST /orders/async  (default)
"""

import os
import json
import random
from locust import HttpUser, task, between

# Switch between sync and async endpoints via env var
TEST_MODE = os.getenv("TEST_MODE", "async")


SAMPLE_ITEMS = [
    {"product_id": "SHIRT-001", "quantity": 1, "price": 29.99},
    {"product_id": "SHOES-042", "quantity": 1, "price": 89.99},
    {"product_id": "HAT-007",   "quantity": 2, "price": 14.99},
    {"product_id": "BAG-013",   "quantity": 1, "price": 49.99},
]


def make_order_payload():
    """Generate a random order payload."""
    num_items = random.randint(1, 3)
    return {
        "customer_id": random.randint(1000, 9999),
        "items": random.sample(SAMPLE_ITEMS, num_items),
    }


class OrderUser(HttpUser):
    # Random wait between requests: simulates real user behavior
    wait_time = between(0.1, 0.5)  # 100-500ms as specified in HW

    @task
    def place_order(self):
        endpoint = f"/orders/{TEST_MODE}"
        payload = make_order_payload()

        with self.client.post(
            endpoint,
            json=payload,
            headers={"Content-Type": "application/json"},
            catch_response=True,
            name=f"POST {endpoint}",
        ) as response:
            if TEST_MODE == "sync":
                # Sync: expect 200 OK after ~3s
                if response.status_code == 200:
                    response.success()
                else:
                    response.failure(
                        f"Sync order failed: {response.status_code} {response.text[:100]}"
                    )
            else:
                # Async: expect 202 Accepted immediately (<100ms)
                if response.status_code == 202:
                    response.success()
                else:
                    response.failure(
                        f"Async order failed: {response.status_code} {response.text[:100]}"
                    )