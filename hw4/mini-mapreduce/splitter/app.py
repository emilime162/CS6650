import os, re, uuid
import boto3
from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI()
s3 = boto3.client("s3")

class SplitReq(BaseModel):
    bucket: str
    key: str
    num_chunks: int = 3

def read_s3_text(bucket: str, key: str) -> str:
    obj = s3.get_object(Bucket=bucket, Key=key)
    return obj["Body"].read().decode("utf-8", errors="ignore")

def put_s3_text(bucket: str, key: str, text: str) -> None:
    s3.put_object(Bucket=bucket, Key=key, Body=text.encode("utf-8"))

@app.post("/split")
def split(req: SplitReq):
    text = read_s3_text(req.bucket, req.key)
    run_id = uuid.uuid4().hex[:8]

    n = max(1, req.num_chunks)
    size = len(text)
    chunk_size = (size + n - 1) // n

    chunk_urls = []
    for i in range(n):
        start = i * chunk_size
        end = min(size, (i + 1) * chunk_size)
        chunk_text = text[start:end]

        out_key = f"chunks/run-{run_id}/chunk-{i}.txt"
        put_s3_text(req.bucket, out_key, chunk_text)
        chunk_urls.append(f"s3://{req.bucket}/{out_key}")

    return {"run_id": run_id, "chunk_urls": chunk_urls}
