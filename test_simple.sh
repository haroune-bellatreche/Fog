#!/bin/bash

# Simple Fog Computing Test Script using curl
# Tests fog nodes without requiring Python

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

NODES=("8081" "8082" "8083")

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}Fog Computing Simple Test Suite${NC}"
echo -e "${GREEN}================================${NC}\n"

# Test 1: Health Checks
echo -e "${YELLOW}Test 1: Health Checks${NC}"
for port in "${NODES[@]}"; do
    response=$(curl -s -w "%{http_code}" -o /tmp/health_$port.json "http://localhost:$port/health")
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✓ Node on port $port is healthy${NC}"
        cat /tmp/health_$port.json | jq . 2>/dev/null || cat /tmp/health_$port.json
    else
        echo -e "${RED}✗ Node on port $port failed (HTTP $response)${NC}"
    fi
done
echo ""

# Test 2: Node Status
echo -e "${YELLOW}Test 2: Node Status${NC}"
for port in "${NODES[@]}"; do
    response=$(curl -s "http://localhost:$port/status")
    echo -e "${GREEN}Node on port $port:${NC}"
    echo "$response" | jq . 2>/dev/null || echo "$response"
    echo ""
done

# Test 3: Submit Tasks
echo -e "${YELLOW}Test 3: Task Submission${NC}"
for port in "${NODES[@]}"; do
    echo -e "${GREEN}Submitting task to node on port $port${NC}"
    
    task_response=$(curl -s -X POST "http://localhost:$port/tasks" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "data_aggregation",
            "payload": {"sensors": [1,2,3], "interval": 60},
            "priority": 1
        }')
    
    echo "$task_response" | jq . 2>/dev/null || echo "$task_response"
    
    # Extract task ID
    task_id=$(echo "$task_response" | jq -r '.id' 2>/dev/null)
    
    if [ ! -z "$task_id" ] && [ "$task_id" != "null" ]; then
        echo -e "${GREEN}Task submitted: $task_id${NC}"
        
        # Wait for task to complete
        sleep 1
        
        # Check task status
        echo -e "${YELLOW}Checking task status...${NC}"
        task_status=$(curl -s "http://localhost:$port/tasks/$task_id")
        echo "$task_status" | jq . 2>/dev/null || echo "$task_status"
    fi
    echo ""
done

# Test 4: Metrics
echo -e "${YELLOW}Test 4: Metrics Collection${NC}"
for port in "${NODES[@]}"; do
    echo -e "${GREEN}Metrics for node on port $port:${NC}"
    metrics=$(curl -s "http://localhost:$port/metrics")
    echo "$metrics" | jq . 2>/dev/null || echo "$metrics"
    echo ""
done

# Test 5: Load Test (submit multiple tasks)
echo -e "${YELLOW}Test 5: Simple Load Test (10 tasks per node)${NC}"
for port in "${NODES[@]}"; do
    echo -e "${GREEN}Submitting 10 tasks to node on port $port${NC}"
    
    for i in {1..10}; do
        curl -s -X POST "http://localhost:$port/tasks" \
            -H "Content-Type: application/json" \
            -d "{
                \"type\": \"preprocessing\",
                \"payload\": {\"test_id\": $i},
                \"priority\": 1
            }" > /dev/null &
    done
    
    wait
    echo -e "${GREEN}✓ 10 tasks submitted${NC}"
    
    # Wait and check metrics
    sleep 2
    metrics=$(curl -s "http://localhost:$port/metrics")
    echo -e "${YELLOW}Updated metrics:${NC}"
    echo "$metrics" | jq . 2>/dev/null || echo "$metrics"
    echo ""
done

# Test 6: Different Task Types
echo -e "${YELLOW}Test 6: Testing Different Task Types${NC}"
port=${NODES[0]}

task_types=("data_aggregation" "edge_analytics" "preprocessing" "caching")

for task_type in "${task_types[@]}"; do
    echo -e "${GREEN}Testing task type: $task_type${NC}"
    
    response=$(curl -s -X POST "http://localhost:$port/tasks" \
        -H "Content-Type: application/json" \
        -d "{
            \"type\": \"$task_type\",
            \"payload\": {\"test\": \"$task_type\"},
            \"priority\": 1
        }")
    
    task_id=$(echo "$response" | jq -r '.id' 2>/dev/null)
    echo "  Task ID: $task_id"
    
    # Wait and check result
    sleep 0.5
    result=$(curl -s "http://localhost:$port/tasks/$task_id")
    status=$(echo "$result" | jq -r '.status' 2>/dev/null)
    echo "  Status: $status"
    echo ""
done

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}All tests completed!${NC}"
echo -e "${GREEN}================================${NC}"
