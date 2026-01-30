#!/bin/bash

# Fog Computing Deployment Script
# This script automates the build, deployment, and testing process

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}Fog Computing Deployment Script${NC}"
echo -e "${GREEN}================================${NC}\n"

# Function to check if Docker is running
check_docker() {
    echo -e "${YELLOW}Checking Docker...${NC}"
    if ! docker info > /dev/null 2>&1; then
        echo -e "${RED}Error: Docker is not running${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Docker is running${NC}\n"
}

# Function to build images
build_images() {
    echo -e "${YELLOW}Building Docker images...${NC}"
    docker-compose build
    echo -e "${GREEN}✓ Images built successfully${NC}\n"
}

# Function to start containers
start_containers() {
    echo -e "${YELLOW}Starting fog computing nodes...${NC}"
    docker-compose up -d
    echo -e "${GREEN}✓ Containers started${NC}\n"
    
    echo -e "${YELLOW}Waiting for nodes to be ready...${NC}"
    sleep 8
    echo -e "${GREEN}✓ Nodes should be ready${NC}\n"
}

# Function to check node health
check_health() {
    echo -e "${YELLOW}Checking node health...${NC}"
    
    nodes=("8081" "8082" "8083")
    all_healthy=true
    
    for port in "${nodes[@]}"; do
        if curl -s -f "http://localhost:$port/health" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Node on port $port is healthy${NC}"
        else
            echo -e "${RED}✗ Node on port $port is not responding${NC}"
            all_healthy=false
        fi
    done
    
    echo ""
    
    if [ "$all_healthy" = true ]; then
        echo -e "${GREEN}All nodes are healthy!${NC}\n"
        return 0
    else
        echo -e "${RED}Some nodes are not healthy${NC}\n"
        return 1
    fi
}

# Function to display node status
show_status() {
    echo -e "${YELLOW}Node Status:${NC}"
    docker-compose ps
    echo ""
}

# Function to run tests
run_tests() {
    echo -e "${YELLOW}Installing Python dependencies...${NC}"
    pip3 install requests -q 2>/dev/null || pip install requests -q 2>/dev/null || true
    
    echo -e "${YELLOW}Running test suite...${NC}"
    python3 test_fog.py
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  deploy    - Build and deploy fog nodes"
    echo "  test      - Run the test suite"
    echo "  full      - Deploy and test (default)"
    echo "  stop      - Stop all nodes"
    echo "  restart   - Restart all nodes"
    echo "  logs      - Show logs"
    echo "  clean     - Stop and remove all containers"
    echo ""
}

# Main script logic
case "${1:-full}" in
    deploy)
        check_docker
        build_images
        start_containers
        show_status
        check_health
        ;;
    test)
        check_health
        run_tests
        ;;
    full)
        check_docker
        build_images
        start_containers
        show_status
        if check_health; then
            run_tests
        else
            echo -e "${RED}Skipping tests due to unhealthy nodes${NC}"
            exit 1
        fi
        ;;
    stop)
        echo -e "${YELLOW}Stopping nodes...${NC}"
        docker-compose down
        echo -e "${GREEN}✓ Nodes stopped${NC}"
        ;;
    restart)
        echo -e "${YELLOW}Restarting nodes...${NC}"
        docker-compose down
        docker-compose up -d
        sleep 5
        check_health
        ;;
    logs)
        docker-compose logs -f
        ;;
    clean)
        echo -e "${YELLOW}Cleaning up...${NC}"
        docker-compose down -v
        docker system prune -f
        echo -e "${GREEN}✓ Cleanup complete${NC}"
        ;;
    help|--help|-h)
        show_usage
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}\n"
        show_usage
        exit 1
        ;;
esac
