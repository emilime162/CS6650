# mapper/app.py
import json, re
import boto3
from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI()
s3 = boto3.client("s3")

class MapReq(BaseModel):
    chunk_url: str

def parse_s3_url(url: str):
    # s3://bucket/key
    assert url.startswith("s3://")
    parts = url[5:].split("/", 1)
    return parts[0], parts[1]

def read_s3_text(bucket: str, key: str) -> str:
    obj = s3.get_object(Bucket=bucket, Key=key)
    return obj["Body"].read().decode("utf-8", errors="ignore")

def put_s3_json(bucket: str, key: str, data: dict) -> None:
    s3.put_object(Bucket=bucket, Key=key, Body=json.dumps(data).encode("utf-8"))

def tokenize(text: str):
    return re.findall(r"[a-zA-Z']+", text.lower())

@app.post("/map")
def do_map(req: MapReq):
    b, k = parse_s3_url(req.chunk_url)
    text = read_s3_text(b, k)
    words = tokenize(text)

    counts = {}
    for w in words:
        counts[w] = counts.get(w, 0) + 1

    # chunks/run-XXXX/chunk-0.txt
    run_id = k.split("/")[1].replace("run-", "")
    chunk_name = k.split("/")[-1]  # chunk-0.txt
    idx = chunk_name.replace("chunk-", "").replace(".txt", "")

    out_key = f"maps/run-{run_id}/map-{idx}.json"
    put_s3_json(b, out_key, counts)

    return {"map_url": f"s3://{b}/{out_key}"}
