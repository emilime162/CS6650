"""
Run exactly 150 operations (50 create, 50 add items, 50 get cart)
and save results to mysql_test_results.json and dynamodb_test_results.json

Usage:
    python run_tests.py --host http://YOUR-ALB-DNS --db mysql
    python run_tests.py --host http://YOUR-ALB-DNS --db dynamo
"""

import argparse
import json
import random
import time
import requests
from datetime import datetime, timezone

def run_tests(host: str, db: str) -> list:
    results = []
    cart_ids = []
    prefix = f"/{db}/shopping-carts"

    print(f"\nRunning 150 operations against {db}...")

    # ── 50 CREATE CART ────────────────────────────────────────────────────────
    print("  Creating 50 carts...")
    for i in range(50):
        start = time.time()
        try:
            resp = requests.post(
                f"{host}{prefix}",
                json={"customer_id": random.randint(1, 9999)},
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000  # ms
            success = resp.status_code == 201
            if success:
                cart_ids.append(resp.json()["shopping_cart_id"])
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            success = False
            resp = type("r", (), {"status_code": 500})()
            print(f"    create error: {e}")

        results.append({
            "operation": "create_cart",
            "response_time": round(elapsed, 2),
            "success": success,
            "status_code": resp.status_code,
            "timestamp": datetime.now(timezone.utc).isoformat(),
        })

    # ── 50 ADD ITEMS ──────────────────────────────────────────────────────────
    print("  Adding items to 50 carts...")
    for i in range(50):
        cart_id = random.choice(cart_ids) if cart_ids else "invalid"
        start = time.time()
        try:
            resp = requests.post(
                f"{host}{prefix}/{cart_id}/items",
                json={
                    "product_id": random.randint(1, 100),
                    "quantity": random.randint(1, 5),
                },
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000
            success = resp.status_code == 204
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            success = False
            resp = type("r", (), {"status_code": 500})()
            print(f"    add_items error: {e}")

        results.append({
            "operation": "add_items",
            "response_time": round(elapsed, 2),
            "success": success,
            "status_code": resp.status_code,
            "timestamp": datetime.now(timezone.utc).isoformat(),
        })

    # ── 50 GET CART ───────────────────────────────────────────────────────────
    print("  Getting 50 carts...")
    for i in range(50):
        cart_id = random.choice(cart_ids) if cart_ids else "invalid"
        start = time.time()
        try:
            resp = requests.get(
                f"{host}{prefix}/{cart_id}",
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000
            success = resp.status_code == 200
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            success = False
            resp = type("r", (), {"status_code": 500})()
            print(f"    get_cart error: {e}")

        results.append({
            "operation": "get_cart",
            "response_time": round(elapsed, 2),
            "success": success,
            "status_code": resp.status_code,
            "timestamp": datetime.now(timezone.utc).isoformat(),
        })

    return results


def print_summary(results: list, db: str):
    total = len(results)
    success = sum(1 for r in results if r["success"])
    times = [r["response_time"] for r in results]
    avg = sum(times) / len(times)

    by_op = {}
    for r in results:
        op = r["operation"]
        if op not in by_op:
            by_op[op] = []
        by_op[op].append(r["response_time"])

    print(f"\n{'='*50}")
    print(f"  {db.upper()} Results Summary")
    print(f"{'='*50}")
    print(f"  Total:    {total} operations")
    print(f"  Success:  {success}/{total} ({100*success//total}%)")
    print(f"  Avg:      {avg:.1f}ms")
    print(f"  Min:      {min(times):.1f}ms")
    print(f"  Max:      {max(times):.1f}ms")
    print(f"\n  By operation:")
    for op, op_times in by_op.items():
        print(f"    {op}: avg {sum(op_times)/len(op_times):.1f}ms")
    print(f"{'='*50}\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", required=True, help="ALB DNS e.g. http://my-alb.elb.amazonaws.com")
    parser.add_argument("--db", required=True, choices=["mysql", "dynamo"], help="mysql or dynamo")
    args = parser.parse_args()

    results = run_tests(args.host, args.db)
    print_summary(results, args.db)

    filename = f"{args.db}{'db' if args.db == 'dynamo' else ''}_test_results.json"
    # HW requires specific filenames
    if args.db == "mysql":
        filename = "mysql_test_results.json"
    else:
        filename = "dynamodb_test_results.json"

    with open(filename, "w") as f:
        json.dump(results, f, indent=2)

    print(f"Results saved to {filename}")