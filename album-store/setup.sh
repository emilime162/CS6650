#!/usr/bin/env bash
# setup.sh — creates DynamoDB tables and S3 bucket for album-store.
# Run once before deploying.  Safe to re-run (ignores "already exists" errors).
set -euo pipefail

REGION=${AWS_REGION:-us-east-1}
ALBUMS_TABLE=${ALBUMS_TABLE:-albums}
PHOTOS_TABLE=${PHOTOS_TABLE:-photos}

# Generate a unique bucket name if S3_BUCKET is not already set.
if [[ -z "${S3_BUCKET:-}" ]]; then
  S3_BUCKET="album-store-photos-$(date +%s)"
fi

echo "=== Album Store AWS Setup ==="
echo "Region:       $REGION"
echo "Albums table: $ALBUMS_TABLE"
echo "Photos table: $PHOTOS_TABLE"
echo "S3 bucket:    $S3_BUCKET"
echo ""

# ── DynamoDB: albums ──────────────────────────────────────────────────────────
echo "[1/5] Creating DynamoDB table: $ALBUMS_TABLE"
aws dynamodb create-table \
  --table-name "$ALBUMS_TABLE" \
  --attribute-definitions AttributeName=album_id,AttributeType=S \
  --key-schema AttributeName=album_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region "$REGION" 2>/dev/null && echo "  created." || echo "  already exists, skipping."

# ── DynamoDB: photos ──────────────────────────────────────────────────────────
echo "[2/5] Creating DynamoDB table: $PHOTOS_TABLE"
aws dynamodb create-table \
  --table-name "$PHOTOS_TABLE" \
  --attribute-definitions \
    AttributeName=album_id,AttributeType=S \
    AttributeName=photo_id,AttributeType=S \
  --key-schema \
    AttributeName=album_id,KeyType=HASH \
    AttributeName=photo_id,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --region "$REGION" 2>/dev/null && echo "  created." || echo "  already exists, skipping."

# ── S3 bucket ─────────────────────────────────────────────────────────────────
echo "[3/5] Creating S3 bucket: $S3_BUCKET"
if [[ "$REGION" == "us-east-1" ]]; then
  aws s3api create-bucket --bucket "$S3_BUCKET" --region "$REGION" 2>/dev/null \
    && echo "  created." || echo "  already exists, skipping."
else
  aws s3api create-bucket --bucket "$S3_BUCKET" --region "$REGION" \
    --create-bucket-configuration LocationConstraint="$REGION" 2>/dev/null \
    && echo "  created." || echo "  already exists, skipping."
fi

# ── Disable Block Public Access ───────────────────────────────────────────────
echo "[4/5] Disabling Block Public Access on $S3_BUCKET"
aws s3api put-public-access-block \
  --bucket "$S3_BUCKET" \
  --public-access-block-configuration \
    BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false
echo "  done."

# ── Bucket policy: public read ────────────────────────────────────────────────
echo "[5/5] Setting public-read bucket policy on $S3_BUCKET"
POLICY=$(cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Sid": "PublicRead",
    "Effect": "Allow",
    "Principal": "*",
    "Action": "s3:GetObject",
    "Resource": "arn:aws:s3:::${S3_BUCKET}/*"
  }]
}
EOF
)
echo "$POLICY" | aws s3api put-bucket-policy --bucket "$S3_BUCKET" --policy file:///dev/stdin
echo "  done."

echo ""
echo "=== Setup complete! ==="
echo ""
echo "Export these on your EC2 instance (or add to album-store.service):"
echo ""
echo "  export AWS_REGION=$REGION"
echo "  export ALBUMS_TABLE=$ALBUMS_TABLE"
echo "  export PHOTOS_TABLE=$PHOTOS_TABLE"
echo "  export S3_BUCKET=$S3_BUCKET"