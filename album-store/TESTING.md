# Album Store Testing Guide

Complete guide for testing your album-store implementation before submitting to ChaosArena.

---

## 📋 Table of Contents

1. [Quick Deployment](#quick-deployment)
2. [Manual API Testing](#manual-api-testing)
3. [Load Testing](#load-testing)
4. [Submit to ChaosArena](#submit-to-chaosarena)
5. [Check Leaderboard](#check-leaderboard)
6. [Troubleshooting](#troubleshooting)

---

## 🚀 Quick Deployment

### Option 1: Terraform (Recommended)

```bash
# 1. Configure variables
cd terraform
cp terraform.tfvars.example terraform.tfvars
nano terraform.tfvars

# Set these required values:
# - s3_bucket_name = "album-store-photos-yourname-12345"
# - key_name = "your-aws-key-name"

# 2. Deploy infrastructure
terraform init
terraform plan
terraform apply

# 3. Get your API endpoint
terraform output base_url
# → http://3.80.123.45:8080

# 4. Deploy the application
cd ..
make build-linux
make deploy HOST=ec2-user@$(terraform -chdir=terraform output -raw instance_public_ip) KEY=~/.ssh/your-key.pem

# 5. Verify service is running
ssh -i ~/.ssh/your-key.pem ec2-user@$(terraform -chdir=terraform output -raw instance_public_ip) "sudo systemctl status album-store"
```

### Option 2: Manual Setup

```bash
# 1. Create AWS resources
./setup.sh

# Note the export lines at the end - you'll need them

# 2. Build the application
make build-linux

# 3. Deploy to EC2 (assuming you created an instance manually)
make deploy HOST=ec2-user@<your-ec2-ip> KEY=~/.ssh/your-key.pem

# 4. SSH and start service
ssh -i ~/.ssh/your-key.pem ec2-user@<your-ec2-ip>
sudo systemctl start album-store
sudo systemctl status album-store
```

---

## 🧪 Manual API Testing

Save your base URL:
```bash
export BASE_URL="http://your-ec2-ip:8080"
```

### Test 1: Health Check ✅

```bash
curl -v $BASE_URL/health
```

**Expected:**
```json
HTTP/1.1 200 OK
Content-Type: application/json

{"status":"ok"}
```

---

### Test 2: Create Album ✅

```bash
curl -X PUT $BASE_URL/albums/test-album-1 \
  -H "Content-Type: application/json" \
  -d '{
    "album_id": "test-album-1",
    "title": "My Test Album",
    "description": "Testing album creation",
    "owner": "you@northeastern.edu"
  }'
```

**Expected:**
```json
HTTP/1.1 200 OK

{
  "album_id": "test-album-1",
  "title": "My Test Album",
  "description": "Testing album creation",
  "owner": "you@northeastern.edu"
}
```

---

### Test 3: Get Album ✅

```bash
curl $BASE_URL/albums/test-album-1
```

**Expected:**
```json
{
  "album_id": "test-album-1",
  "title": "My Test Album",
  "description": "Testing album creation",
  "owner": "you@northeastern.edu"
}
```

---

### Test 4: List Albums ✅

```bash
curl $BASE_URL/albums
```

**Expected:**
```json
[
  {
    "album_id": "test-album-1",
    "title": "My Test Album",
    "description": "Testing album creation",
    "owner": "you@northeastern.edu"
  }
]
```

---

### Test 5: Upload Photo (Async) ✅

```bash
# Create a test image
curl -o test-photo.jpg https://via.placeholder.com/300

# Upload it
curl -X POST $BASE_URL/albums/test-album-1/photos \
  -F "photo=@test-photo.jpg" \
  -v
```

**Expected:**
```json
HTTP/1.1 202 Accepted

{
  "photo_id": "f1e2d3c4-...",
  "seq": 1,
  "status": "processing"
}
```

**Save the photo_id for next tests!**

---

### Test 6: Check Photo Status ✅

```bash
# Replace <photo_id> with the ID from previous response
export PHOTO_ID="<photo_id>"

# Check status (might be processing)
curl $BASE_URL/albums/test-album-1/photos/$PHOTO_ID

# Wait a few seconds and try again - should be completed
sleep 3
curl $BASE_URL/albums/test-album-1/photos/$PHOTO_ID
```

**Expected (processing):**
```json
{
  "photo_id": "f1e2d3c4-...",
  "album_id": "test-album-1",
  "seq": 1,
  "status": "processing"
}
```

**Expected (completed):**
```json
{
  "photo_id": "f1e2d3c4-...",
  "album_id": "test-album-1",
  "seq": 1,
  "status": "completed",
  "url": "https://your-bucket.s3.us-east-1.amazonaws.com/photos/f1e2d3c4-..."
}
```

---

### Test 7: Verify Photo URL ✅

```bash
# Extract URL from previous response and test it
export PHOTO_URL="<url_from_previous_response>"
curl -I $PHOTO_URL
```

**Expected:**
```
HTTP/1.1 200 OK
Content-Type: image/jpeg
```

---

### Test 8: Delete Photo ✅

```bash
curl -X DELETE $BASE_URL/albums/test-album-1/photos/$PHOTO_ID -v
```

**Expected:**
```
HTTP/1.1 204 No Content
```

**Verify deletion:**
```bash
# GET should return 404
curl $BASE_URL/albums/test-album-1/photos/$PHOTO_ID
# → {"error":"not found"}

# URL should no longer return 200
curl -I $PHOTO_URL
# → HTTP/1.1 403 Forbidden (or 404)
```

---

### Test 9: Test Sequence Numbers ✅

```bash
# Upload 3 photos to the same album
curl -X POST $BASE_URL/albums/test-album-1/photos -F "photo=@test-photo.jpg"
curl -X POST $BASE_URL/albums/test-album-1/photos -F "photo=@test-photo.jpg"
curl -X POST $BASE_URL/albums/test-album-1/photos -F "photo=@test-photo.jpg"

# All should return seq: 2, 3, 4 (assuming we deleted seq 1)
```

**Expected:**
```json
{"photo_id":"...","seq":2,"status":"processing"}
{"photo_id":"...","seq":3,"status":"processing"}
{"photo_id":"...","seq":4,"status":"processing"}
```

---

### Test 10: Test Per-Album Sequences ✅

```bash
# Create a second album
curl -X PUT $BASE_URL/albums/test-album-2 \
  -H "Content-Type: application/json" \
  -d '{"album_id":"test-album-2","title":"Second Album","description":"Test","owner":"you@northeastern.edu"}'

# Upload to second album
curl -X POST $BASE_URL/albums/test-album-2/photos -F "photo=@test-photo.jpg"
```

**Expected:**
```json
{"photo_id":"...","seq":1,"status":"processing"}
```

✅ **seq should be 1 (independent counter per album)**

---

## 🔥 Load Testing

Before submitting to ChaosArena, do some basic load testing:

### Test Concurrent Album Creates

```bash
# Install apache bench (if needed)
# macOS: brew install httpd
# Linux: sudo apt-get install apache2-utils

# Create test payload
cat > album.json <<EOF
{"album_id":"test-album-load","title":"Load Test","description":"Testing","owner":"test@test.com"}
EOF

# 100 concurrent requests
ab -n 100 -c 10 -T "application/json" -p album.json -m PUT $BASE_URL/albums/test-album-load

# Check p95 latency in the results
```

### Test Concurrent Photo Uploads

```bash
# Simple parallel uploads
for i in {1..10}; do
  curl -X POST $BASE_URL/albums/test-album-1/photos \
    -F "photo=@test-photo.jpg" &
done
wait

# Check all completed
curl $BASE_URL/albums/test-album-1/photos | jq
```

### Monitor Service During Load

```bash
# In a separate terminal, SSH to EC2 and watch logs
ssh -i ~/.ssh/your-key.pem ec2-user@<ip>
sudo journalctl -u album-store -f
```

Look for:
- ❌ Errors or panics
- ❌ Slow response times
- ✅ Clean log output with no errors

---

## 🎯 Submit to ChaosArena

### 1. Check the Leaderboard First

```bash
curl https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/leaderboard | jq
```

**Expected:**
```json
[
  {
    "rank": 1,
    "nickname": "Tiger",
    "score": 189,
    "correctness_score": 110,
    "load_score": 79
  },
  {
    "rank": 2,
    "nickname": "Panda",
    "score": 175,
    "correctness_score": 105,
    "load_score": 70
  }
]
```

This shows you what scores other students are getting!

---

### 2. Submit Your Service

```bash
export BASE_URL="http://your-ec2-ip:8080"

curl -X POST https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/submit \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"you@northeastern.edu\",
    \"nickname\": \"YourNickname\",
    \"base_url\": \"$BASE_URL\",
    \"contract\": \"v1-album-store\"
  }" | jq
```

**Expected:**
```json
{
  "run_id": "abc123def456",
  "status": "queued",
  "message": "Submission received"
}
```

**Save the run_id!**

---

### 3. Poll for Results

```bash
export RUN_ID="abc123def456"

# Check status
curl https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/runs/$RUN_ID | jq

# Keep checking until status is "completed" (takes 5-10 minutes)
watch -n 10 "curl -s https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/runs/$RUN_ID | jq '.status'"
```

**Status progression:**
```
queued → running → completed
```

---

### 4. View Detailed Results

```bash
curl https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/runs/$RUN_ID | jq '.' > results.json

# View in a nice format
cat results.json | jq
```

**Result structure:**
```json
{
  "run_id": "abc123def456",
  "status": "completed",
  "score": 185,
  "correctness_score": 110,
  "load_score": 75,
  "scenarios": [
    {
      "name": "S1_HEALTH_CHECK",
      "status": "PASSED",
      "points_awarded": 5,
      "events": [...]
    },
    {
      "name": "S11_CONCURRENT_CREATES_LOAD",
      "status": "PASSED",
      "points_awarded": 14,
      "metrics": {
        "p95_ms": 187,
        "p99_ms": 312,
        "error_rate": 0.002
      }
    }
  ]
}
```

---

### 5. Check Updated Leaderboard

```bash
curl https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/leaderboard | jq
```

You should now see your nickname and score!

---

## 📊 Understanding Your Score

### Correctness (110 pts max)

Look at scenarios S1-S10:
```bash
cat results.json | jq '.scenarios[] | select(.name | startswith("S")) | {name, status, points_awarded}'
```

**What to look for:**
- ✅ All S1-S5 must PASS (critical scenarios)
- ✅ S10 (sequence numbers) is worth 15 points

### Load (80 pts max)

Look at scenarios S11-S15:
```bash
cat results.json | jq '.scenarios[] | select(.name | contains("LOAD")) | {name, points_awarded, metrics}'
```

**What to look for:**
- Your p95 vs reference p95
- Error rate (should be < 1%)
- S15 (large uploads) is worth 20 points

---

## 🐛 Troubleshooting

### Service Won't Start

```bash
ssh -i ~/.ssh/your-key.pem ec2-user@<ip>

# Check service status
sudo systemctl status album-store

# Check logs
sudo journalctl -u album-store -n 100

# Check if binary exists
ls -la /home/ec2-user/album-store

# Check environment variables
sudo systemctl show album-store | grep Environment
```

### Photos Stuck in "processing"

```bash
# Check worker logs
ssh -i ~/.ssh/your-key.pem ec2-user@<ip>
sudo journalctl -u album-store | grep worker

# Check S3 permissions
aws s3 ls s3://your-bucket-name/photos/

# Test S3 upload manually
aws s3 cp test-photo.jpg s3://your-bucket-name/test.jpg
```

### Connection Timeouts

```bash
# Check security group allows port 8080
aws ec2 describe-security-groups --group-ids <sg-id> | jq '.SecurityGroups[].IpPermissions'

# Should see port 8080 open to 0.0.0.0/0
```

### DynamoDB Errors

```bash
# Check IAM role has permissions
ssh -i ~/.ssh/your-key.pem ec2-user@<ip>
aws sts get-caller-identity
aws dynamodb list-tables

# Should see your tables: albums, photos
```

### ChaosArena Says "Service Unreachable"

```bash
# Test from external network
curl http://your-ec2-ip:8080/health

# If this fails, check:
# 1. EC2 instance is running
# 2. Service is running (systemctl status)
# 3. Security group allows port 8080
# 4. You're using PUBLIC IP, not private IP
```

---

## 📈 Tips for Maximum Score

### Before Submitting

1. ✅ Run all manual tests above
2. ✅ Do basic load testing with `ab`
3. ✅ Check logs for errors
4. ✅ Verify S3 bucket is public-readable
5. ✅ Test from external network (not SSH tunnel)

### During Testing

1. 🔍 Monitor logs in real-time:
   ```bash
   ssh -i ~/.ssh/your-key.pem ec2-user@<ip> "sudo journalctl -u album-store -f"
   ```

2. 📊 Watch system resources:
   ```bash
   ssh -i ~/.ssh/your-key.pem ec2-user@<ip> "htop"
   ```

### After First Submission

1. 📋 Read the event logs carefully
2. 🔧 Fix any FAILED scenarios
3. 🚀 Optimize slow scenarios (high p95 latency)
4. 🔄 Submit again (your highest score is kept!)

---

## 🎯 Quick Reference Commands

```bash
# Deploy with Terraform
cd terraform && terraform apply && cd ..
make build-linux
make deploy HOST=ec2-user@$(terraform -chdir=terraform output -raw instance_public_ip) KEY=~/.ssh/key.pem

# Test all endpoints
export BASE_URL="http://$(terraform -chdir=terraform output -raw instance_public_ip):8080"
curl $BASE_URL/health
curl -X PUT $BASE_URL/albums/test -H "Content-Type: application/json" -d '{"album_id":"test","title":"Test","description":"Test","owner":"test@test.com"}'
curl -X POST $BASE_URL/albums/test/photos -F "photo=@test.jpg"

# Submit to ChaosArena
curl -X POST https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/submit \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"you@northeastern.edu\",\"nickname\":\"YourName\",\"base_url\":\"$BASE_URL\",\"contract\":\"v1-album-store\"}"

# Check leaderboard
curl https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/leaderboard | jq
```

---

## 🏆 Good Luck!

Your implementation looks solid. Expected score: **173-190 points (91-100%)**

Remember:
- You can submit multiple times
- Your highest score is kept
- Check the leaderboard to see where you stand
- Read the event logs to improve your score
