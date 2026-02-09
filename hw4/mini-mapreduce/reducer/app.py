# reducer/app.py
import json
import boto3
from fastapi import FastAPI
from pydantic import BaseModel
from typing import List

app = FastAPI()
s3 = boto3.client("s3")

class ReduceReq(BaseModel):
    map_urls: List[str]

def parse_s3_url(url: str):
    assert url.startswith("s3://")
    parts = url[5:].split("/", 1)
    return parts[0], parts[1]

def read_s3_json(bucket: str, key: str) -> dict:
    obj = s3.get_object(Bucket=bucket, Key=key)
    return json.loads(obj["Body"].read().decode("utf-8"))

def put_s3_json(bucket: str, key: str, data: dict) -> None:
    s3.put_object(Bucket=bucket, Key=key, Body=json.dumps(data).encode("utf-8"))

@app.post("/reduce")
def reduce(req: ReduceReq):
    merged = {}
    run_id = None
    bucket = None

    for url in req.map_urls:
        b, k = parse_s3_url(url)
        bucket = b
        # maps/run-XXXX/map-0.json
        run_id = k.split("/")[1].replace("run-", "")
        d = read_s3_json(b, k)
        for w, c in d.items():
            merged[w] = merged.get(w, 0) + int(c)

    out_key = f"reduce/run-{run_id}/result.json"
    put_s3_json(bucket, out_key, merged)
    return {"result_url": f"s3://{bucket}/{out_key}"}
