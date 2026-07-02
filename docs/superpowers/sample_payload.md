{
  "sourceContext": "payment-service-prod",
  "signals": [
    {
      "id": "sig-001",
      "eventType": "log",
      "source": "payment-api",
      "environment": "prod",
      "timestamp": "2026-06-29T10:15:30Z",
      "severity": 3,
      "message": "Payment retry triggered for order 4821",
      "payload": {
        "requestId": "req-123",
        "orderId": "ord-4821",
        "retryCount": 2
      },
      "metadata": {
        "team": "payments",
        "region": "ap-southeast-1"
      }
    }
  ]
}


{
  "sourceContext": "checkout-platform",
  "signals": [
    {
      "eventType": "deployment",
      "source": "deploy-bot",
      "environment": "staging",
      "timestamp": "2026-06-29T09:00:00Z",
      "severity": 2,
      "message": "Deployed checkout-service version 1.14.2",
      "payload": {
        "service": "checkout-service",
        "version": "1.14.2",
        "commit": "a1b2c3d4"
      },
      "metadata": {
        "pipeline": "github-actions"
      }
    },
    {
      "eventType": "database",
      "source": "orders-db",
      "environment": "staging",
      "timestamp": "2026-06-29T09:03:10Z",
      "severity": 4,
      "message": "Database connection pool saturation detected",
      "payload": {
        "dbName": "orders",
        "activeConnections": 95,
        "maxConnections": 100
      },
      "metadata": {
        "cluster": "postgres-staging"
      }
    },
    {
      "eventType": "queue",
      "source": "rabbitmq-worker",
      "environment": "staging",
      "timestamp": "2026-06-29T09:05:45Z",
      "severity": 3,
      "message": "Queue backlog increased above threshold",
      "payload": {
        "queueName": "order-events",
        "depth": 1820,
        "consumerCount": 3
      },
      "metadata": {
        "owner": "async-processing"
      }
    }
  ]
}


{
  "sourceContext": "inventory-monitoring",
  "signals": [
    {
      "eventType": "health",
      "source": "inventory-service",
      "environment": "prod",
      "timestamp": "2026-06-29T11:20:00Z",
      "severity": 1,
      "message": "Health check latency elevated",
      "payload": {
        "endpoint": "/health",
        "latencyMs": 850,
        "statusCode": 200
      },
      "metadata": {
        "region": "eu-west-1"
      }
    },
    {
      "eventType": "log",
      "source": "inventory-service",
      "environment": "prod",
      "timestamp": "2026-06-29T11:21:10Z",
      "severity": 5,
      "message": "Out of memory crash in inventory sync worker",
      "payload": {
        "worker": "sync-worker-7",
        "node": "inventory-node-02"
      },
      "metadata": {
        "service": "inventory"
      }
    }
  ]
}