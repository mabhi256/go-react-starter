# Prod deployment: AWS target

No live infrastructure is provisioned here. This directory documents the intended AWS
architecture and provides reference artifacts. IaC (Terraform / CDK) is a follow-up.

## Architecture

```text
Internet
  └─ ALB (HTTPS, ACM cert)
       └─ ECS Fargate Cluster
            ├─ Service: api       (image: <ECR>/xyzmed-backend, entrypoint /app/api)
            └─ Service: worker    (image: <ECR>/xyzmed-backend, entrypoint /app/worker)
                  │
                  ├─ Amazon RDS (Postgres 16, Multi-AZ)
                  └─ Amazon ElastiCache (Valkey, cluster mode)

Frontend → AWS Amplify Hosting (build from frontend/, CDN auto-provisioned)
Email    → Amazon SES (verified domain, IAM task role permission)
Secrets  → AWS Secrets Manager / SSM Parameter Store (injected as ECS env vars)
```

## ECS task definition: api service (reference)

```json
{
  "family": "xyzmed-api",
  "cpu": "512",
  "memory": "1024",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "containerDefinitions": [
    {
      "name": "api",
      "image": "<ACCOUNT>.dkr.ecr.ap-south-1.amazonaws.com/xyzmed-backend:latest",
      "essential": true,
      "portMappings": [{ "containerPort": 8080, "protocol": "tcp" }],
      "environment": [
        { "name": "APP_ENV", "value": "prod" },
        { "name": "HTTP_PORT", "value": "8080" },
        { "name": "OTEL_SERVICE_NAME", "value": "xyzmed-api" }
      ],
      "secrets": [
        { "name": "DB_DSN",        "valueFrom": "arn:aws:ssm:...:xyzmed/DB_DSN" },
        { "name": "REDIS_ADDR",    "valueFrom": "arn:aws:ssm:...:xyzmed/REDIS_ADDR" },
        { "name": "JWT_SECRET",    "valueFrom": "arn:aws:ssm:...:xyzmed/JWT_SECRET" },
        { "name": "GOOGLE_OAUTH_CLIENT_ID", "valueFrom": "arn:aws:ssm:...:xyzmed/GOOGLE_CLIENT_ID" }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/xyzmed-api",
          "awslogs-region": "ap-south-1",
          "awslogs-stream-prefix": "ecs"
        }
      }
    }
  ]
}
```

Override `entrypoint` to `["/app/worker"]` for the worker service.

## Amplify build settings

```yaml
# amplify.yml
version: 1
frontend:
  phases:
    preBuild:
      commands:
        - npm install -g bun
        - bun install --frozen-lockfile
    build:
      commands:
        - bun run build
  artifacts:
    baseDirectory: dist
    files:
      - "**/*"
  cache:
    paths:
      - node_modules/**/*
```

Environment variables to set in the Amplify Console:

- `VITE_API_URL` = `https://api.yourdomain.com`

## CI/CD: Docker build cache

The `--mount=type=cache` mounts in the Dockerfiles are local-only (they persist on the
build host between runs). On ephemeral CI runners (GitHub Actions, etc.) the cache is
cold on every run and the mounts do nothing.

To get warm build caches in CI, wire up BuildKit's cache export/import in the workflow:

```yaml
- uses: docker/build-push-action@v5
  with:
    cache-from: type=gha
    cache-to: type=gha,mode=max
```

This persists the BuildKit layer cache to the GHA cache store between runs. Without it,
the `--mount=type=cache` lines are no-ops in CI but cost nothing to leave in place.

## Prod env-var checklist

| Key | Source |
|-----|--------|
| `APP_ENV` | `prod` (hardcoded in task def) |
| `DB_DSN` | SSM: RDS cluster writer endpoint |
| `REDIS_ADDR` | SSM: ElastiCache primary endpoint |
| `REDIS_PASSWORD` | SSM: ElastiCache auth token |
| `JWT_SECRET` | Secrets Manager: 32+ byte random |
| `JWT_ACCESS_TTL` | `15m` |
| `JWT_REFRESH_TTL` | `720h` |
| `GOOGLE_OAUTH_CLIENT_ID` | SSM: GCP console client ID |
| `MAIL_FROM` | `no-reply@yourdomain.com` |
| `AWS_REGION` | `ap-south-1` |
| `SMS_PROVIDER` | TBD (add MSG91/Twilio adapter) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | AWS Distro for OpenTelemetry collector endpoint |
| `OTEL_SERVICE_NAME` | `xyzmed-api` / `xyzmed-worker` |
