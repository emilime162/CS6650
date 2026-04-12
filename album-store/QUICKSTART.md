# Album Store Quick Start

## 🚀 Deploy in 5 Minutes

### 1. Configure & Deploy
```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars - set these 2 required values:
# - s3_bucket_name = "album-store-photos-yourname-12345"
# - key_name = "your-aws-key-name"

terraform init
terraform apply
```

### 2. Upload Application
```bash
cd ..
make build-linux

# Get the EC2 IP
export EC2_IP=$(terraform -chdir=terraform output -raw instance_public_ip)

# Deploy
make deploy HOST=ec2-user@$EC2_IP KEY=~/.ssh/your-key.pem
```

### 3. Test It
```bash
export BASE_URL="http://$EC2_IP:8080"

# Run automated tests
./test.sh $BASE_URL

# Or test manually
curl $BASE_URL/health
# → {"status":"ok"}
```

### 4. Submit to ChaosArena
```bash
curl -X POST https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/submit \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"you@northeastern.edu\",
    \"nickname\": \"YourNickname\",
    \"base_url\": \"$BASE_URL\",
    \"contract\": \"v1-album-store\"
  }"

# Save the run_id from response
```

### 5. Check Results
```bash
# Replace with your run_id
export RUN_ID="your-run-id"

# Poll for completion (takes 5-10 minutes)
watch -n 10 "curl -s https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/runs/$RUN_ID | jq '.status, .score'"

# View full results
curl https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/runs/$RUN_ID | jq '.' > results.json
```

### 6. Check Leaderboard
```bash
curl https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/leaderboard | jq
```

---

## 🐛 Quick Troubleshooting

### Service won't start?
```bash
ssh -i ~/.ssh/your-key.pem ec2-user@$EC2_IP
sudo journalctl -u album-store -n 50
```

### Can't connect to API?
```bash
# Check security group allows port 8080
# Check you're using PUBLIC IP
curl http://$EC2_IP:8080/health
```

### Photos stuck in "processing"?
```bash
# Check worker logs
ssh -i ~/.ssh/your-key.pem ec2-user@$EC2_IP
sudo journalctl -u album-store | grep worker
```

---

## 📚 More Info

- **Full Testing Guide**: See [TESTING.md](TESTING.md)
- **Terraform Guide**: See [terraform/README.md](terraform/README.md)
- **Architecture**: See [README.md](README.md)

---

## 🎯 Expected Score: 173-190 points (91-100%)

Good luck! 🚀
