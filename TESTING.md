# Fog Computing Testing Guide

This document provides comprehensive testing methodologies for the fog computing system.

## Table of Contents
1. [Quick Testing](#quick-testing)
2. [Automated Testing](#automated-testing)
3. [Manual Testing](#manual-testing)
4. [Performance Testing](#performance-testing)
5. [Integration Testing](#integration-testing)
6. [Monitoring and Debugging](#monitoring-and-debugging)

---

## Quick Testing

### Method 1: Using Make Commands

```bash
# Build and start nodes
make build
make run

# Quick health check
make quick-test

# Full test suite
make test

# View logs
make logs
```

### Method 2: Using Deployment Script

```bash
# Full deployment and testing
./deploy.sh full

# Just deploy
./deploy.sh deploy

# Just test
./deploy.sh test

# Restart and test
./deploy.sh restart
```

### Method 3: Simple Bash Tests

```bash
# Run simple curl-based tests
./test_simple.sh
```

---

## Automated Testing

### Python Test Suite (`test_fog.py`)

The comprehensive Python test suite includes:

#### 1. Health Check Tests
- Verifies all nodes are responding
- Measures response times
- Validates health endpoint structure

```bash
python3 test_fog.py
```

**What it tests:**
- Node availability
- Response time < 5 seconds
- Correct JSON response format

#### 2. Node Status Tests
- Retrieves node configuration
- Validates node metadata
- Checks load information

**Expected output:**
```json
{
  "id": "fog-node-1",
  "location": "edge-site-1",
  "status": "active",
  "load": 0.15
}
```

#### 3. Task Submission Tests
- Tests all task types
- Validates task lifecycle
- Checks result correctness

**Task types tested:**
- `data_aggregation`
- `edge_analytics`
- `preprocessing`
- `caching`

#### 4. Concurrent Load Tests
- Submits 50 simultaneous tasks
- Measures throughput
- Calculates latency percentiles

**Metrics collected:**
- Total tasks processed
- Success/failure rate
- Average latency
- P95 latency
- Throughput (tasks/second)

#### 5. Latency Distribution Tests
- Tests each task type separately
- Multiple samples per type
- Statistical analysis

**Statistics calculated:**
- Mean latency
- Median latency
- Standard deviation
- Min/Max values

#### 6. Metrics Collection Tests
- Validates metrics endpoint
- Checks metric accuracy
- Verifies real-time updates

---

## Manual Testing

### Basic Tests

#### Test 1: Health Check
```bash
curl http://localhost:8081/health
```

**Expected response:**
```json
{
  "status": "healthy",
  "node": "fog-node-1"
}
```

#### Test 2: Node Status
```bash
curl http://localhost:8081/status
```

**Expected response:**
```json
{
  "id": "fog-node-1",
  "location": "edge-site-1",
  "status": "active",
  "load": 0.05,
  "last_seen": "2026-01-29T10:30:00Z"
}
```

#### Test 3: Submit Simple Task
```bash
curl -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "preprocessing",
    "payload": {"data": "test"},
    "priority": 1
  }'
```

**Expected response:**
```json
{
  "id": "task-1706534400123456789",
  "type": "preprocessing",
  "payload": {"data": "test"},
  "priority": 1,
  "status": "queued",
  "submitted_at": "2026-01-29T10:30:00Z"
}
```

#### Test 4: Check Task Status
```bash
# Use task ID from previous response
curl http://localhost:8081/tasks/task-1706534400123456789
```

**Expected response (completed):**
```json
{
  "id": "task-1706534400123456789",
  "type": "preprocessing",
  "status": "completed",
  "result": {
    "operation": "preprocessing",
    "status": "success",
    "filtered": true,
    "normalized": true
  },
  "submitted_at": "2026-01-29T10:30:00Z",
  "completed_at": "2026-01-29T10:30:00.150Z"
}
```

### Advanced Manual Tests

#### Test All Task Types

**Data Aggregation:**
```bash
curl -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "data_aggregation",
    "payload": {
      "sensors": [1, 2, 3, 4, 5],
      "interval": 60
    },
    "priority": 1
  }'
```

**Edge Analytics:**
```bash
curl -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "edge_analytics",
    "payload": {
      "data_points": 100,
      "algorithm": "anomaly_detection"
    },
    "priority": 2
  }'
```

**Preprocessing:**
```bash
curl -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "preprocessing",
    "payload": {
      "data": "raw_sensor_data",
      "filter": "kalman"
    },
    "priority": 1
  }'
```

**Caching:**
```bash
curl -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "caching",
    "payload": {
      "key": "sensor_data_123",
      "value": {"temperature": 25.5, "humidity": 60}
    },
    "priority": 1
  }'
```

---

## Performance Testing

### Load Testing with Apache Bench

```bash
# Install Apache Bench (if needed)
sudo apt-get install apache2-utils

# Create test payload
echo '{
  "type": "preprocessing",
  "payload": {"test": "load"},
  "priority": 1
}' > task.json

# Run load test (100 requests, 10 concurrent)
ab -n 100 -c 10 -p task.json -T application/json \
  http://localhost:8081/tasks
```

### Load Testing with wrk

```bash
# Install wrk
sudo apt-get install wrk

# Create Lua script for POST requests
cat > post.lua << 'EOF'
wrk.method = "POST"
wrk.body   = '{"type":"preprocessing","payload":{"test":"load"},"priority":1}'
wrk.headers["Content-Type"] = "application/json"
EOF

# Run test (12 threads, 400 connections, 30 seconds)
wrk -t12 -c400 -d30s -s post.lua http://localhost:8081/tasks
```

### Stress Testing Script

```bash
#!/bin/bash
# Submit 1000 tasks as fast as possible

for i in {1..1000}; do
  curl -X POST http://localhost:8081/tasks \
    -H "Content-Type: application/json" \
    -d "{\"type\":\"preprocessing\",\"payload\":{\"id\":$i},\"priority\":1}" \
    > /dev/null 2>&1 &
  
  # Limit concurrent requests
  if [ $((i % 50)) -eq 0 ]; then
    wait
  fi
done

wait
echo "1000 tasks submitted"
```

### Latency Testing

```bash
#!/bin/bash
# Measure latency for 100 requests

total=0
count=100

for i in $(seq 1 $count); do
  start=$(date +%s%N)
  
  curl -s -X POST http://localhost:8081/tasks \
    -H "Content-Type: application/json" \
    -d '{"type":"preprocessing","payload":{"test":true},"priority":1}' \
    > /dev/null
  
  end=$(date +%s%N)
  latency=$((($end - $start) / 1000000))
  total=$(($total + $latency))
  
  echo "Request $i: ${latency}ms"
done

avg=$(($total / $count))
echo "Average latency: ${avg}ms"
```

---

## Integration Testing

### Multi-Node Testing

Test load balancing across nodes:

```bash
#!/bin/bash
# Round-robin task submission to all nodes

nodes=("8081" "8082" "8083")
tasks_per_node=20

for node in "${nodes[@]}"; do
  echo "Submitting tasks to node on port $node"
  
  for i in $(seq 1 $tasks_per_node); do
    curl -s -X POST "http://localhost:$node/tasks" \
      -H "Content-Type: application/json" \
      -d "{\"type\":\"preprocessing\",\"payload\":{\"node\":\"$node\",\"id\":$i},\"priority\":1}" \
      > /dev/null &
  done
done

wait

# Check metrics on all nodes
for node in "${nodes[@]}"; do
  echo -e "\nMetrics for node on port $node:"
  curl -s "http://localhost:$node/metrics" | jq .
done
```

### Network Partition Testing

Simulate network issues:

```bash
# Stop one node
docker stop fog-node-2

# Continue sending tasks to remaining nodes
curl -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{"type":"preprocessing","payload":{"test":true},"priority":1}'

# Restart node
docker start fog-node-2

# Verify it rejoins
curl http://localhost:8082/health
```

---

## Monitoring and Debugging

### Real-time Monitoring

```bash
# Watch logs from all nodes
docker-compose logs -f

# Watch specific node
docker logs -f fog-node-1

# Monitor resource usage
docker stats fog-node-1 fog-node-2 fog-node-3

# Watch metrics in real-time
watch -n 1 'curl -s http://localhost:8081/metrics | jq .'
```

### Debugging Failed Tasks

```bash
# 1. Check node status
curl http://localhost:8081/status

# 2. Check metrics
curl http://localhost:8081/metrics

# 3. Submit test task and monitor
TASK_ID=$(curl -s -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{"type":"preprocessing","payload":{"debug":true},"priority":1}' \
  | jq -r '.id')

echo "Task ID: $TASK_ID"

# 4. Poll task status
for i in {1..10}; do
  sleep 1
  curl -s "http://localhost:8081/tasks/$TASK_ID" | jq .
done

# 5. Check logs for errors
docker logs fog-node-1 | grep ERROR
```

### Performance Profiling

```bash
# Check current load
for port in 8081 8082 8083; do
  echo "Node on port $port:"
  curl -s "http://localhost:$port/status" | jq '.load'
done

# Monitor queue size (via logs)
docker logs fog-node-1 2>&1 | grep "queue"

# Check processing times
docker logs fog-node-1 2>&1 | grep "completed in"
```

---

## Test Scenarios

### Scenario 1: Normal Operation
1. Start all nodes
2. Submit mixed task types
3. Verify all complete successfully
4. Check metrics are reasonable

### Scenario 2: High Load
1. Submit 100+ concurrent tasks
2. Monitor success rate
3. Check latency stays acceptable
4. Verify no crashes

### Scenario 3: Node Failure
1. Stop one node
2. Verify other nodes continue
3. Restart failed node
4. Verify it recovers

### Scenario 4: Task Priority
1. Submit low priority tasks
2. Submit high priority tasks
3. Verify processing order (future feature)

### Scenario 5: Long Running Tasks
1. Modify task processing time
2. Submit long-running tasks
3. Verify timeout handling
4. Check graceful degradation

---

## Expected Performance Baselines

Based on typical hardware:

- **Health Check Response**: < 50ms
- **Task Submission**: < 100ms
- **Task Processing**: 30-200ms depending on type
- **Throughput**: 50-100 tasks/second per node
- **P95 Latency**: < 500ms
- **Success Rate**: > 99%

---

## Continuous Testing

### Automated Test Schedule

```bash
# Create cron job for periodic testing
# Run tests every hour
0 * * * * cd /path/to/fog-compute && ./test_simple.sh >> /var/log/fog-test.log 2>&1

# Daily comprehensive test
0 2 * * * cd /path/to/fog-compute && python3 test_fog.py >> /var/log/fog-test-daily.log 2>&1
```

### CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: Fog Computing Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v2
    
    - name: Build Docker images
      run: docker-compose build
    
    - name: Start services
      run: docker-compose up -d
    
    - name: Wait for services
      run: sleep 10
    
    - name: Run tests
      run: python3 test_fog.py
    
    - name: Cleanup
      run: docker-compose down
```

---

## Troubleshooting Tests

### Tests Timeout
- Increase sleep delays between checks
- Reduce concurrent task count
- Check system resources

### Connection Refused
- Verify nodes are running: `docker ps`
- Check ports are available: `netstat -tulpn`
- Review firewall settings

### Incorrect Results
- Check task implementation in `main.go`
- Verify JSON parsing
- Review task status transitions

### High Failure Rate
- Check system resources
- Reduce worker count
- Increase task queue size
- Review logs for errors

---

## Conclusion

This testing guide provides multiple approaches to validate the fog computing system:

1. **Quick validation**: Health checks and simple tests
2. **Comprehensive testing**: Full automated test suite
3. **Performance testing**: Load and stress tests
4. **Integration testing**: Multi-node scenarios
5. **Continuous monitoring**: Real-time observation

Choose the appropriate testing method based on your needs and environment.
