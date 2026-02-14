import random
from locust import HttpUser, task, between
from locust.contrib.fasthttp import FastHttpUser

PRODUCTS = [
    {
        "product_id": i,
        "sku": f"SKU-{i:04d}",
        "manufacturer": f"Manufacturer-{i}",
        "category_id": random.randint(1, 50),
        "weight": random.randint(100, 5000),
        "some_other_id": random.randint(1, 1000),
    }
    for i in range(1, 101)
]

PRODUCT_COUNTER = 100

class HttpUserLoadTest(HttpUser):
    wait_time = between(1, 3)

    def on_start(self):
        for product in PRODUCTS:
            self.client.post(
                f"/products/{product['product_id']}/details",
                json=product,
                name="/products/[id]/details (seed)",
            )

    @task(9)
    def get_product(self):
        product_id = random.randint(1, 100)
        self.client.get(f"/products/{product_id}", name="/products/[id]")

    @task(1)
    def add_product(self):
        global PRODUCT_COUNTER
        PRODUCT_COUNTER += 1
        pid = PRODUCT_COUNTER
        product = {
            "product_id": pid,
            "sku": f"SKU-{pid:04d}",
            "manufacturer": f"Manufacturer-{pid}",
            "category_id": random.randint(1, 50),
            "weight": random.randint(100, 5000),
            "some_other_id": random.randint(1, 1000),
        }
        self.client.post(
            f"/products/{pid}/details",
            json=product,
            name="/products/[id]/details",
        )

class FastHttpUserLoadTest(FastHttpUser):
    wait_time = between(1, 3)

    def on_start(self):
        for product in PRODUCTS:
            self.client.post(
                f"/products/{product['product_id']}/details",
                json=product,
                name="/products/[id]/details (seed)",
            )

    @task(9)
    def get_product(self):
        product_id = random.randint(1, 100)
        self.client.get(f"/products/{product_id}", name="/products/[id]")

    @task(1)
    def add_product(self):
        global PRODUCT_COUNTER
        PRODUCT_COUNTER += 1
        pid = PRODUCT_COUNTER
        product = {
            "product_id": pid,
            "sku": f"SKU-{pid:04d}",
            "manufacturer": f"Manufacturer-{pid}",
            "category_id": random.randint(1, 50),
            "weight": random.randint(100, 5000),
            "some_other_id": random.randint(1, 1000),
        }
        self.client.post(
            f"/products/{pid}/details",
            json=product,
            name="/products/[id]/details",
        )
