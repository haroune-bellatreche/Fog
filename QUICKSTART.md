# Quick Start Guide - Fog Computing in Docker

## What You Have

A complete fog computing system with:
- **3 Go-based fog nodes** running in Docker
- **RESTful API** for task submission and monitoring
- **4 task types**: Data Aggregation, Edge Analytics, Preprocessing, Caching
- **Comprehensive testing suite** (Python + Bash)
- **Auto-deployment scripts**

## Files Overview

```
fog-compute/
├── main.go              # Go application (fog computing logic)
├── go.mod               # Go dependencies
├── Dockerfile           # Container image definition
├── docker-compose.yml   # Multi-node orchestration
├── Makefile            # Convenient commands
├── deploy.sh           # Automated deployment script
├── test_fog.py         # Comprehensive Python test suite
├── test_simple.sh      # Simple bash tests (no Python needed)
├── README.md           # Full documentation
└── TESTING.md          # Testing methodology guide
```

## 3-Step Quick Start

### Step 1: Build and Deploy
```bash
# Option A: Using deployment script (recommended)
chmod +x deploy.sh
./deploy.sh deploy

# Option B: Using Make
make build
make run

# Option C: Using Docker Compose directly
docker-compose build
docker-compose up -d
```

### Step 2: Verify It's Running
```bash
# Quick health check
curl http://localhost:8081/health
curl http://localhost:8082/health
curl http://localhost:8083/health

# Or use make
make status
```

Expected output:
```json
{"status":"healthy","node":"fog-node-1"}
```

### Step 3: Test It
```bash
# Option A: Simple bash tests (no dependencies)
chmod +x test_simple.sh
./test_simple.sh

# Option B: Comprehensive Python tests
pip3 install requests
python3 test_fog.py

# Option C: Using Make
make test
```

## Try It Out Manually

### Submit a Task
```bash
curl -X POST http://localhost:8081/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "edge_analytics",
    "payload": {"data_points": 100},
    "priority": 1
  }'
```

You'll get back a task ID like:
```json
{
  "id": "task-1706534400123456789",
  "type": "edge_analytics",
  "status": "queued",
  ...
}
```

### Check Task Status
```bash
# Replace with your actual task ID
curl http://localhost:8081/tasks/task-1706534400123456789
```

Result:
```json
{
  "id": "task-1706534400123456789",
  "status": "completed",
  "result": {
    "operation": "edge_analytics",
    "status": "success",
    "insights": "Anomaly detected in sensor readings",
    "confidence": 0.87
  }
}
```

### View Metrics
```bash
curl http://localhost:8081/metrics
```

## Common Commands

```bash
# Start nodes
make run
# or
./deploy.sh deploy

# Stop nodes
make stop
# or
docker-compose down

# View logs
make logs
# or
docker-compose logs -f

# Run tests
make test

# Restart everything
make restart

# Clean up everything
make clean
```

## The 4 Task Types

1. **data_aggregation** - Aggregates sensor data from multiple sources
   ```bash
   curl -X POST http://localhost:8081/tasks \
     -H "Content-Type: application/json" \
     -d '{"type":"data_aggregation","payload":{"sensors":[1,2,3]},"priority":1}'
   ```

2. **edge_analytics** - Performs analytical computations
   ```bash
   curl -X POST http://localhost:8081/tasks \
     -H "Content-Type: application/json" \
     -d '{"type":"edge_analytics","payload":{"algorithm":"anomaly"},"priority":1}'
   ```

3. **preprocessing** - Filters and normalizes data
   ```bash
   curl -X POST http://localhost:8081/tasks \
     -H "Content-Type: application/json" \
     -d '{"type":"preprocessing","payload":{"filter":"kalman"},"priority":1}'
   ```

4. **caching** - Caches data for faster access
   ```bash
   curl -X POST http://localhost:8081/tasks \
     -H "Content-Type: application/json" \
     -d '{"type":"caching","payload":{"key":"data_123","value":{}},"priority":1}'
   ```

## Node Configuration

The system runs 3 fog nodes by default:

- **fog-node-1**: http://localhost:8081
- **fog-node-2**: http://localhost:8082
- **fog-node-3**: http://localhost:8083

Each node has:
- 5 concurrent workers
- 100-task queue capacity
- Health monitoring
- Metrics collection

## API Endpoints

All nodes expose:
- `GET /health` - Health check
- `GET /status` - Node status and load
- `GET /metrics` - Performance metrics
- `POST /tasks` - Submit new task
- `GET /tasks/{id}` - Get task status/result

## Troubleshooting

### Nodes won't start?
```bash
# Check if ports are free
lsof -i :8081
lsof -i :8082
lsof -i :8083

# Check Docker
docker ps -a
docker-compose logs
```

### Can't connect?
```bash
# Verify containers are running
docker ps

# Check if services are listening
curl http://localhost:8081/health
```

### Tests fail?
```bash
# Wait for nodes to fully start
sleep 10
# Then retry tests
make test
```

## What's Next?

1. **Read the full documentation**: `README.md`
2. **Explore testing methods**: `TESTING.md`
3. **Modify the code**: Edit `main.go` to add features
4. **Scale up**: Add more nodes in `docker-compose.yml`
5. **Integrate**: Connect to your IoT devices or applications

## Architecture Diagram

```
┌─────────────────────────────────────────────────┐
│            Client Applications / IoT            │
└──────────────────┬──────────────────────────────┘
                   │
        ┌──────────┼──────────┐
        │          │          │
   ┌────▼───┐ ┌───▼────┐ ┌───▼────┐
   │ Node 1 │ │ Node 2 │ │ Node 3 │
   │ :8081  │ │ :8082  │ │ :8083  │
   └────────┘ └────────┘ └────────┘
   
   Each node processes:
   - Data Aggregation
   - Edge Analytics
   - Preprocessing
   - Caching
```

## Performance Expectations

- **Task Latency**: 30-200ms per task
- **Throughput**: 50-100 tasks/sec per node
- **Concurrent Tasks**: 5 per node (configurable)
- **Success Rate**: >99%

## Use Cases

✓ IoT data processing at the edge
✓ Video analytics before cloud upload
✓ Smart city traffic analysis
✓ Industrial sensor monitoring
✓ Content delivery and caching
✓ Real-time data preprocessing

---

**Need help?** Check `README.md` for detailed docs or `TESTING.md` for testing strategies.

**Ready to deploy?** Just run: `./deploy.sh full`
