# Album Store Assignment Report
**Student:** Emily Chen (chen.shix@northeastern.edu)  
**Nickname:** rocky  
**Date:** April 12, 2026

---

## 1. Roughly how many submissions did it take before you passed all critical scenarios, and what was the most common failure?

**Answer:** It took 1 submission to pass all critical scenarios (S1-S5) and achieve 100/110 correctness. However, reaching the final score of 179/190 took multiple iterations:

**Progression:**
- Submission 1 (single instance): 163/190 - passed all critical scenarios but S9 failed (race condition)
- Submission 2-3 (with load balancer): 167/190 - improved load handling but S9 still failing
- Submission 4+ (S9 fix applied): 179/190 - fixed race condition, achieved perfect correctness

**Most common failure:** The **S9_DELETE_BEFORE_COMPLETE** scenario was the most challenging. The race condition where async workers would resurrect deleted photos required adding conditional DynamoDB updates. This single bug fix was worth 12 points.

---

## 2. Where are your photo files stored, and why did you pick that over other options?

**Answer:** Photo files are stored in **Amazon S3** (`album-store-photos-chen-20260412`). I chose S3 because:
- It provides a public URL for each photo, which ChaosArena needs to verify
- It's highly scalable and handles large files efficiently (critical for S15)
- The S3 Transfer Manager automatically uses multipart uploads for files >5MB with concurrent part uploads
- It's cheaper and more reliable than storing files on the EC2 instance itself
- Separating storage from compute makes the system more maintainable

---

## 3. Describe your deployment setup — how many instances, what cloud services, and how they connect to each other.

**Answer:** 
- **4 EC2 instances** (t3.xlarge in us-west-2), each running the Go application as a systemd service with 384 worker goroutines
- **Application Load Balancer** distributing traffic across all 4 instances on port 80, forwarding to instances on port 8080
- **2 DynamoDB tables**: `albums` (hash key: album_id) and `photos` (hash key: album_id, range key: photo_id) using PAY_PER_REQUEST billing
- **1 S3 bucket** with public read policy for storing photo files
- **IAM instance profile** attached to each EC2 with permissions for DynamoDB and S3 operations
- All instances communicate directly with DynamoDB and S3 via AWS SDK using instance profile credentials (no hardcoded keys)
- Security groups configured so instances only accept traffic from the ALB on port 8080

---

## 4. Did you use a reverse proxy or load balancer? If so, what role does it play in your architecture?

**Answer:** Yes, I used an **Application Load Balancer (ALB)**. It plays a critical role:
- **Traffic distribution**: Routes incoming HTTP requests evenly across 4 EC2 instances
- **Health checks**: Continuously monitors `/health` endpoint every 10 seconds; unhealthy instances are automatically removed from rotation
- **High availability**: If an instance fails, the ALB immediately stops sending it traffic
- **Horizontal scaling**: Enabled me to go from 163 points (single instance) to 179 points (4 instances)

The ALB listens on port 80 and forwards to instances on port 8080. It provides the single public endpoint that ChaosArena tests against, abstracting away the multiple backend instances.

---

## 5. How does your background worker get notified that there's a new photo to process? Did you use a queue, polling, or something else?

**Answer:** I use an **in-memory Go channel** as a job queue. The POST handler immediately submits a job to the channel (non-blocking with 4096 buffer size), and 384 worker goroutines per instance continuously read from this channel and process uploads. With 4 instances, that's 1,536 total workers handling uploads concurrently. This avoids the overhead and latency of external queues like SQS while still providing async processing. The handler returns 202 immediately after enqueuing the job, and the worker goroutines handle the S3 upload in the background.

---

## 6. The spec requires that `seq` is assigned in the POST handler, not the background worker. Why does that matter, and how did you ensure correctness under concurrent uploads to the same album?

**Answer:** Assigning `seq` in the POST handler matters because:
- The 202 response must include the `seq` value immediately
- Multiple concurrent uploads to the same album need unique, monotonically increasing sequence numbers
- If workers assigned seq, there would be race conditions

I ensure correctness using **DynamoDB's atomic ADD operation** on a `photo_count` attribute in the albums table. The POST handler calls `UpdateItem` with `ADD photo_count :1` and `RETURN_VALUES=UPDATED_NEW`, which atomically increments the counter and returns the new value as the seq number. This is thread-safe across all concurrent requests.

---

## 7. What happens in your system if the worker crashes or fails halfway through processing a photo?

**Answer:** If a worker goroutine crashes:
- The photo record stays in `status="processing"` indefinitely
- The job is lost from the in-memory queue (not persisted)
- If the entire service crashes, the systemd service auto-restarts within 1 second (configured with `Restart=always` and `RestartSec=1`)

**Limitation:** There's no retry mechanism. A production system would use SQS with visibility timeouts and dead-letter queues, but for this assignment, the in-memory approach provided better latency. The `StartLimitIntervalSec=0` configuration ensures systemd never gives up restarting the service.

---

## 8. What does your database schema look like? What tables or collections did you create and why?

**Answer:**

**Table 1: `albums`**
- Hash key: `album_id` (String)
- Attributes: `title`, `description`, `owner`, `photo_count` (Number, for seq generation)
- Purpose: Store album metadata and maintain per-album photo counter

**Table 2: `photos`**
- Hash key: `album_id` (String)
- Range key: `photo_id` (String)
- Attributes: `seq` (Number), `status` (String), `url` (String, optional)
- Purpose: Store photo metadata and track processing status

I used two separate tables because albums and photos have different access patterns - albums are accessed by ID, while photos need composite queries (album_id + photo_id).

---

## 9. Did you add any indexes to your database? If so, on which columns and why?

**Answer:** No additional indexes were needed. DynamoDB automatically indexes the hash key and range key. All my queries use these keys:
- Get album: query by `album_id` (hash key)
- Get photo: query by `album_id` + `photo_id` (composite key)
- List albums: full table scan (required by spec, no filter possible)

The spec doesn't require listing photos by album, so I didn't need a GSI. For a production system, I would add a GSI on `album_id` in the photos table to support efficient "get all photos for an album" queries.

---

## 10. Which load testing scenario was the hardest for you, and what bottleneck did you discover?

**Answer:** Based on my final score (69/80 on load), the hardest scenarios were **S12 (Concurrent Photo Uploads)** with only 6-7 points out of 20. Analysis shows:
- **S12**: Heavy concurrent photo uploads stress S3 throughput across all instances
- **S14/S15**: Mixed operations and large payloads also lost a few points

The main bottleneck is **aggregate S3 upload bandwidth**. Despite having 4 instances with 384 workers each (1,536 total workers), the load balancer architecture helps CPU and request distribution but S3 uploads are still limited by per-instance network bandwidth. 

Top performers (190/190) likely used:
- Compute-optimized instances (c5.xlarge) with 10 Gbps network vs t3.xlarge's 5 Gbps
- More aggressive S3 Transfer Manager settings
- Possibly 5-6 instances instead of 4

---

## 11. What was the single most impactful change you made to improve your load test scores?

**Answer:** **Fixing the S9 race condition bug (+12 points)** was the most impactful single change, but for continuous improvement, it was **implementing horizontal scaling with a load balancer**.

**Score progression:**
- Single instance baseline: 163/190
- With 4 instances + ALB: 167/190 (+4 points)
- After fixing S9 bug: 179/190 (+12 points)

**Key optimizations that enabled this:**
1. Load balancer architecture with 4x t3.xlarge instances
2. 384 worker goroutines per instance (1,536 total)
3. Connection pooling (2000 max connections per instance):
```go
transport := &http.Transport{
    MaxIdleConns:        2000,
    MaxIdleConnsPerHost: 1000,
    IdleConnTimeout:     90 * time.Second,
    DisableCompression:  true,
}
```
4. Conditional DynamoDB updates to prevent race conditions

The combination of horizontal scaling and race condition fixes took me from 163 to 179 (16-point improvement).

---

## 12. How did you handle concurrent writes — for example, many album creates or photo uploads happening at the same time?

**Answer:**
- **Album creates:** DynamoDB's `PutItem` is naturally idempotent and thread-safe. Multiple PUTs to the same album_id simply overwrite (last write wins), which matches the spec requirement for idempotency.
- **Photo uploads:** Each upload generates a unique UUID for photo_id, so there are no collisions. The seq number is assigned atomically via DynamoDB's ADD operation (see question 6).
- **Connection pooling:** Large connection pools (2000) prevent resource exhaustion
- **Worker pool:** 128 goroutines process uploads concurrently without blocking each other

The Go HTTP server handles concurrency natively with goroutines per request, and DynamoDB handles concurrent writes with optimistic locking.

---

## 13. Describe a specific bug you ran into and how you diagnosed it using the ChaosArena event logs or your own logs.

**Answer:** **Bug (S9_DELETE_BEFORE_COMPLETE failure):** Photos deleted while still processing were resurrecting as "completed" records after deletion, causing orphaned database entries.

**Diagnosis from ChaosArena logs:**
1. S9 scenario showed: photo uploaded (202), immediately deleted (204), then after 2 seconds GET returned 200 instead of 404
2. Event log showed: `VIOLATION: orphaned record: photo d8228716... returned 200 after DELETE — async worker wrote metadata after deletion was confirmed`
3. Root cause: DynamoDB's `UpdateItem` creates the item if it doesn't exist, so the worker was recreating deleted photos

**Fix:** Added conditional update in `UpdatePhotoStatus`:
```go
// Only update if photo still exists and is in "processing" state
conditionExpr := "attribute_exists(photo_id) AND #s = :processing"
```

This prevents the worker from resurrecting deleted photos. The update fails silently if the photo was deleted, which is the correct behavior.

**Impact:** This single fix improved score from 167 to 179 (+12 points) by passing S9.

**Lesson:** Race conditions in async systems require defensive programming. Always validate assumptions before writes.

---

## 14. How did you test your service locally before submitting to ChaosArena?

**Answer:** I created an automated test script (`test.sh`) that runs 12 test scenarios:
- Health check
- Create/Get/List albums
- Upload photo (async) and verify 202 response
- Poll photo status until completed
- Verify photo URL returns 200
- Delete photo and verify 404
- Test sequence numbers (monotonic increment)
- Test per-album sequences (independent counters)

The script uses `curl` and validates JSON responses, HTTP status codes, and timing. I ran this script after every deployment before submitting to ChaosArena. It caught the S3 bucket configuration bug before I submitted.

---

## 15. If you had another week, what is the one thing you would change or add to your system to improve your score?

**Answer:** I would implement **connection pooling to a persistent worker queue** using Redis or SQS instead of in-memory channels. This would provide:
- **Retry logic:** Failed uploads could be retried automatically
- **Persistence:** Jobs survive service restarts
- **Visibility timeout:** Detect stuck workers and reassign jobs
- **Metrics:** Track queue depth and processing time

Currently, if a worker crashes mid-upload, the job is lost. A persistent queue with retries would improve reliability and potentially reduce error rates in load tests. I might also add **CloudWatch metrics** to monitor performance and identify bottlenecks in real-time.

---

## 16. How did you add value over and above what Claude could do in this assignment?

**Answer:** 
- **Infrastructure as Code:** I used Claude to help create the Terraform modules, but I made the architectural decisions (modular structure, us-west-2 region, t3.xlarge instance type)
- **Performance tuning:** I adjusted worker count (128), connection pool sizes (2000), and S3 Transfer Manager settings based on understanding load requirements
- **Debugging:** I diagnosed the S3 bucket configuration issue by analyzing logs and understanding the deployment flow
- **Testing strategy:** I created and ran the comprehensive test script to validate functionality before submission
- **System design decisions:** I chose in-memory queues over SQS for lower latency, atomic DynamoDB operations for correctness, and specific resource limits in the systemd configuration

Claude provided implementation assistance and code generation, but I made the system design decisions, debugged issues, and ensured the complete deployment worked end-to-end.

---

## Final Notes

**Deployed Infrastructure:**
- Base URL: http://album-store-alb-2086233274.us-west-2.elb.amazonaws.com
- Region: us-west-2
- Architecture: Application Load Balancer + 4x EC2 instancesexit
- Instance Type: t3.xlarge
- Worker goroutines: 384 per instance (1,536 total)
- S3 Bucket: album-store-photos-chen-20260412

**Final Score:** 179/190 points (94.2%)

**Breakdown:**
- Correctness: 110/110 (100%) ✅ Perfect!
- Load: 69/80 (86.3%)
- Rank: 36 out of 87 students (Top 41%)

**Key Achievements:**
- Perfect correctness score (all S1-S10 scenarios passed)
- Fixed critical S9 race condition (delete-before-complete)
- Implemented horizontal scaling with load balancer
- Only 11 points away from perfect score (190/190)
- Top 10 students achieved 190; only 10 out of 87 got perfect scores

**What separated me from 190:**
- S12 (Concurrent Photos Load): 6-7/20 points - main bottleneck
- S14/S15: Lost 2-3 points combined on mixed/large payload tests
- Likely needed c5.xlarge instances (10 Gbps network) instead of t3.xlarge (5 Gbps)
- Or 5-6 instances instead of 4 for better aggregate bandwidth
