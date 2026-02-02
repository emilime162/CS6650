from locust import task, between
from locust.contrib.fasthttp import FastHttpUser


class AlbumUser(FastHttpUser):
    # wait for 1-2 seconds
    wait_time = between(1, 2)

    @task(3)
    def get_albums(self):
        with self.client.get("/albums", name="GET /albums", catch_response=True) as r:
            if r.status_code != 200:
                r.failure(f"Expected 200, got {r.status_code}")
            else:
                r.success()

    @task(1)
    def post_album(self):
        payload = {
            "id": "locust-1",
            "title": "Load Test Album",
            "artist": "Locust",
            "price": 12.34
        }
        with self.client.post("/albums", json=payload, name="POST /albums", catch_response=True) as r:
            if r.status_code not in (200, 201):
                r.failure(f"Expected 200/201, got {r.status_code}")
            else:
                r.success()


