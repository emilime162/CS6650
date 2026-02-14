# CS6650 Assignment 2b — Product API

A Product API built in Go with [Chi](https://github.com/go-chi/chi) router, deployed to AWS ECS/ECR using Terraform.

## Project Structure

```
CS6650_2b_demo/
├── src/                          # Server code
│   ├── main.go                   # Entry point, router setup, middleware
│   ├── handlers/
│   │   └── product.go            # HTTP handlers for GET and POST endpoints
│   ├── models/
│   │   └── product.go            # Product and Error structs (matches OpenAPI schema)
│   ├── store/
│   │   └── product.go            # Thread-safe in-memory storage (hashmap + RWMutex)
│   ├── Dockerfile                # Multi-stage build for containerization
│   ├── go.mod                    # Go module dependencies
│   └── go.sum                    # Dependency checksums
├── terraform/                    # Infrastructure as code
│   ├── main.tf                   # Wires modules together, builds & pushes Docker image
│   ├── variables.tf              # Configurable variables (ports, names, etc.)
│   ├── outputs.tf                # Cluster and service names for CLI usage
│   ├── provider.tf               # AWS and Docker provider configuration
│   └── modules/
│       ├── ecr/                  # Container registry
│       ├── ecs/                  # Container orchestration (Fargate)
│       ├── logging/              # CloudWatch log group
│       └── network/              # VPC, subnets, security groups
├── .gitignore                    # Excludes tfstate, binaries, secrets
└── README.MD
```

## Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (must be running)
- [Terraform](https://developer.hashicorp.com/terraform/install)
- [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- AWS Academy Learner Lab credentials

## Deployment Instructions

### 1. Clone the Repository

```bash
git https://github.com/emilime162/CS6650.git
cd hw5
cd CS6650_2b_demo
```

### 2. Configure AWS Credentials

Get your temporary credentials from AWS Learner's Lab, then:

```bash
aws configure
# Enter: Access Key ID, Secret Access Key, Region (us-west-2)

aws configure set aws_session_token <YOUR-SESSION-TOKEN>
```

### 3. Make Sure Docker Desktop Is Running

Open Docker Desktop and wait until it shows "Docker is running." Terraform uses Docker to build and push the container image.

### 4. Deploy with Terraform

```bash
cd terraform
terraform init
terraform apply -auto-approve
```

This will:
- Create an ECR repository
- Build the Docker image from `src/`
- Push the image to ECR
- Create an ECS Fargate cluster and service
- Set up networking and logging

### 5. Get the Public IP

```bash
aws ec2 describe-network-interfaces \
--network-interface-ids $(
    aws ecs describe-tasks \
    --cluster $(terraform output -raw ecs_cluster_name) \
    --tasks $(
        aws ecs list-tasks \
        --cluster $(terraform output -raw ecs_cluster_name) \
        --service-name $(terraform output -raw ecs_service_name) \
        --query 'taskArns[0]' --output text
    ) \
    --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
    --output text
) \
--query 'NetworkInterfaces[0].Association.PublicIp' \
--output text
```

Or find it in the AWS Console: **ECS → Clusters → CS6650L2-cluster → Tasks → (click running task) → Networking tab**.

### 6. Test the API

Replace `<PUBLIC-IP>` with the IP from the previous step:

```bash
curl http://<PUBLIC-IP>:8080/products/1
```

### 7. Clean Up (When Done)

```bash
terraform destroy -auto-approve
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/products/{productId}` | Retrieve a product by ID |
| POST | `/products/{productId}/details` | Add/update product details |

## API Examples — Every Response Code

### POST `/products/{productId}/details`

**204 — Product details added successfully**

```bash
curl -v -X POST http://<PUBLIC-IP>:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "sku": "ABC-123-XYZ",
    "manufacturer": "Acme Corporation",
    "category_id": 456,
    "weight": 1250,
    "some_other_id": 789
  }'
```

Response: `HTTP/1.1 204 No Content` (no response body)

---

**400 — Missing required field**

```bash
curl -v -X POST http://<PUBLIC-IP>:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{"product_id": 1, "sku": "ABC-123-XYZ"}'
```

Response:
```json
{"error": "INVALID_INPUT", "message": "manufacturer is required"}
```

---

**400 — Invalid JSON body**

```bash
curl -v -X POST http://<PUBLIC-IP>:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d 'not json'
```

Response:
```json
{"error": "INVALID_INPUT", "message": "Invalid JSON: invalid character 'o' in literal null (expecting 'u')"}
```

---

**400 — Invalid path parameter (non-integer)**

```bash
curl -v -X POST http://<PUBLIC-IP>:8080/products/abc/details \
  -H "Content-Type: application/json" \
  -d '{}'
```

Response:
```json
{"error": "INVALID_INPUT", "message": "productId must be an integer"}
```

---

**400 — Invalid path parameter (below minimum)**

```bash
curl -v -X POST http://<PUBLIC-IP>:8080/products/0/details \
  -H "Content-Type: application/json" \
  -d '{}'
```

Response:
```json
{"error": "INVALID_INPUT", "message": "productId must be >= 1"}
```

---

### GET `/products/{productId}`

**200 — Product found** *(after creating a product with POST above)*

```bash
curl -v http://<PUBLIC-IP>:8080/products/1
```

Response:
```json
{
  "product_id": 1,
  "sku": "ABC-123-XYZ",
  "manufacturer": "Acme Corporation",
  "category_id": 456,
  "weight": 1250,
  "some_other_id": 789
}
```

---

**404 — Product not found**

```bash
curl -v http://<PUBLIC-IP>:8080/products/999
```

Response:
```json
{"error": "NOT_FOUND", "message": "product with ID 999 not found"}
```

---

**400 — Invalid product ID**

```bash
curl -v http://<PUBLIC-IP>:8080/products/abc
```

Response:
```json
{"error": "INVALID_INPUT", "message": "productId must be an integer"}
```

## Design Decisions

**In-memory hashmap (`map[int]*Product`):** O(1) lookups by product ID — the most common operation in a real store. In production this would be backed by a database.

**`sync.RWMutex`:** Allows concurrent reads (GET) while ensuring exclusive access for writes (POST). Since reads vastly outnumber writes in e-commerce, this is a significant concurrency win over a plain `sync.Mutex`.

**Chi router:** Lightweight and idiomatic Go. Its `{param}` syntax matches OpenAPI path templates directly.

**Multi-stage Docker build:** Final image uses only `alpine`, keeping the container small (~15MB vs ~700MB with the full Go toolchain).

## .gitignore

The following are excluded from the repository:

- **Terraform state:** `*.tfstate`, `.terraform/` (contains sensitive infrastructure details)
- **Secrets:** `.env`, `*.tfvars`, `*.pem`, `*.key`
- **Build artifacts:** Go binaries, Docker artifacts
- **IDE files:** `.vscode/`, `.idea/`

## Troubleshooting

**`exec format error` in ECS logs:** Architecture mismatch. Make sure the Dockerfile has `--platform=linux/amd64` on `FROM` lines or uses `GOARCH=amd64` in the build command.

**`Cannot connect to Docker daemon`:** Start Docker Desktop before running `terraform apply`.

**Terraform shows "no changes" after updating code:** Force a rebuild:
```bash
cd terraform
terraform taint docker_image.app
terraform taint docker_registry_image.app
terraform apply -auto-approve
```

**New image deployed but ECS still runs old version:** Force a new deployment:
```bash
aws ecs update-service \
  --cluster CS6650L2-cluster \
  --service CS6650L2 \
  --force-new-deployment
```