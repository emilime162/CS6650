import os
import random
import json
from locust import HttpUser, task, between

# Switch between mysql and dynamo via env var
DB_MODE = os.getenv("DB_MODE", "mysql")  # "mysql" or "dynamo"

class CartUser(HttpUser):
    wait_time = between(0.1, 0.5)
    cart_ids = []  # shared cart IDs for get/add operations

    def on_start(self):
        # Create a cart on startup so we have IDs to work with
        resp = self.client.post(
            f"/{DB_MODE}/shopping-carts",
            json={"customer_id": random.randint(1, 9999)},
        )
        if resp.status_code == 201:
            self.cart_ids.append(resp.json()["shopping_cart_id"])

    @task(1)
    def create_cart(self):
        self.client.post(
            f"/{DB_MODE}/shopping-carts",
            json={"customer_id": random.randint(1, 9999)},
            name=f"POST /{DB_MODE}/shopping-carts",
        )

    @task(1)
    def add_items(self):
        if not self.cart_ids:
            return
        cart_id = random.choice(self.cart_ids)
        self.client.post(
            f"/{DB_MODE}/shopping-carts/{cart_id}/items",
            json={
                "product_id": random.randint(1, 100),
                "quantity": random.randint(1, 5),
            },
            name=f"POST /{DB_MODE}/shopping-carts/id/items",
        )

    @task(1)
    def get_cart(self):
        if not self.cart_ids:
            return
        cart_id = random.choice(self.cart_ids)
        self.client.get(
            f"/{DB_MODE}/shopping-carts/{cart_id}",
            name=f"GET /{DB_MODE}/shopping-carts/id",
        )