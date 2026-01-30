#!/usr/bin/env python3
"""
Fog Computing Test Suite
Tests fog computing nodes for functionality, performance, and reliability
"""

import requests
import json
import time
import concurrent.futures
from typing import Dict, List, Tuple
import statistics
from datetime import datetime


class FogComputeTester:
    def __init__(self, base_urls: List[str]):
        """Initialize tester with fog node URLs"""
        self.base_urls = base_urls
        self.results = {
            "health_checks": [],
            "task_submissions": [],
            "load_tests": [],
            "latency_tests": []
        }

    def test_health_check(self) -> Dict:
        """Test health endpoint of all nodes"""
        print("\n=== Testing Health Checks ===")
        results = []
        
        for url in self.base_urls:
            try:
                start = time.time()
                response = requests.get(f"{url}/health", timeout=5)
                elapsed = time.time() - start
                
                result = {
                    "node": url,
                    "status": response.status_code == 200,
                    "response_time": elapsed,
                    "data": response.json() if response.status_code == 200 else None
                }
                
                print(f"✓ {url}: {'PASS' if result['status'] else 'FAIL'} ({elapsed:.3f}s)")
                results.append(result)
                
            except Exception as e:
                print(f"✗ {url}: FAIL - {str(e)}")
                results.append({
                    "node": url,
                    "status": False,
                    "error": str(e)
                })
        
        self.results["health_checks"] = results
        return results

    def test_node_status(self) -> Dict:
        """Test status endpoint to get node information"""
        print("\n=== Testing Node Status ===")
        results = []
        
        for url in self.base_urls:
            try:
                response = requests.get(f"{url}/status", timeout=5)
                data = response.json()
                
                print(f"Node: {data.get('id')}")
                print(f"  Location: {data.get('location')}")
                print(f"  Status: {data.get('status')}")
                print(f"  Load: {data.get('load'):.2%}")
                
                results.append({
                    "node": url,
                    "data": data,
                    "success": True
                })
                
            except Exception as e:
                print(f"✗ {url}: Failed to get status - {str(e)}")
                results.append({
                    "node": url,
                    "success": False,
                    "error": str(e)
                })
        
        return results

    def submit_task(self, url: str, task_type: str, payload: Dict) -> Tuple[bool, Dict]:
        """Submit a single task to a fog node"""
        try:
            task = {
                "type": task_type,
                "payload": payload,
                "priority": 1
            }
            
            response = requests.post(f"{url}/tasks", json=task, timeout=10)
            
            if response.status_code == 200:
                return True, response.json()
            else:
                return False, {"error": f"Status code: {response.status_code}"}
                
        except Exception as e:
            return False, {"error": str(e)}

    def test_task_submission(self) -> Dict:
        """Test task submission and completion"""
        print("\n=== Testing Task Submission ===")
        results = []
        
        task_types = [
            ("data_aggregation", {"sensors": [1, 2, 3], "interval": 60}),
            ("edge_analytics", {"data_points": 100, "algorithm": "anomaly_detection"}),
            ("preprocessing", {"data": "raw_sensor_data", "filter": "kalman"}),
            ("caching", {"key": "sensor_data_123", "value": {"temp": 25.5}})
        ]
        
        for url in self.base_urls:
            node_results = []
            
            for task_type, payload in task_types:
                success, data = self.submit_task(url, task_type, payload)
                
                if success:
                    task_id = data.get('id')
                    print(f"✓ {url}: Submitted {task_type} (ID: {task_id})")
                    
                    # Wait a bit and check task status
                    time.sleep(0.5)
                    task_status = self.check_task_status(url, task_id)
                    
                    node_results.append({
                        "task_type": task_type,
                        "success": True,
                        "task_id": task_id,
                        "status": task_status
                    })
                else:
                    print(f"✗ {url}: Failed to submit {task_type} - {data.get('error')}")
                    node_results.append({
                        "task_type": task_type,
                        "success": False,
                        "error": data.get('error')
                    })
            
            results.append({
                "node": url,
                "tasks": node_results
            })
        
        self.results["task_submissions"] = results
        return results

    def check_task_status(self, url: str, task_id: str) -> Dict:
        """Check the status of a submitted task"""
        try:
            response = requests.get(f"{url}/tasks/{task_id}", timeout=5)
            if response.status_code == 200:
                return response.json()
            else:
                return {"error": f"Status code: {response.status_code}"}
        except Exception as e:
            return {"error": str(e)}

    def test_concurrent_load(self, num_tasks: int = 50) -> Dict:
        """Test concurrent task submission (load test)"""
        print(f"\n=== Testing Concurrent Load ({num_tasks} tasks) ===")
        results = []
        
        for url in self.base_urls:
            print(f"\nTesting {url}...")
            start_time = time.time()
            successful = 0
            failed = 0
            latencies = []
            
            def submit_concurrent_task(i):
                task_start = time.time()
                success, data = self.submit_task(
                    url,
                    "preprocessing",
                    {"data": f"test_data_{i}"}
                )
                latency = time.time() - task_start
                return success, latency
            
            with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
                futures = [executor.submit(submit_concurrent_task, i) for i in range(num_tasks)]
                
                for future in concurrent.futures.as_completed(futures):
                    success, latency = future.result()
                    if success:
                        successful += 1
                        latencies.append(latency)
                    else:
                        failed += 1
            
            total_time = time.time() - start_time
            
            result = {
                "node": url,
                "total_tasks": num_tasks,
                "successful": successful,
                "failed": failed,
                "total_time": total_time,
                "throughput": num_tasks / total_time,
                "avg_latency": statistics.mean(latencies) if latencies else 0,
                "min_latency": min(latencies) if latencies else 0,
                "max_latency": max(latencies) if latencies else 0,
                "p95_latency": statistics.quantiles(latencies, n=20)[18] if len(latencies) > 20 else 0
            }
            
            print(f"  Successful: {successful}/{num_tasks}")
            print(f"  Failed: {failed}/{num_tasks}")
            print(f"  Total time: {total_time:.2f}s")
            print(f"  Throughput: {result['throughput']:.2f} tasks/s")
            print(f"  Avg latency: {result['avg_latency']:.3f}s")
            print(f"  P95 latency: {result['p95_latency']:.3f}s")
            
            results.append(result)
        
        self.results["load_tests"] = results
        return results

    def test_metrics_collection(self) -> Dict:
        """Test metrics endpoint"""
        print("\n=== Testing Metrics Collection ===")
        results = []
        
        for url in self.base_urls:
            try:
                response = requests.get(f"{url}/metrics", timeout=5)
                data = response.json()
                
                print(f"\nMetrics for {url}:")
                print(f"  Tasks Processed: {data.get('tasks_processed')}")
                print(f"  Avg Latency: {data.get('avg_latency_ms')}ms")
                print(f"  Current Load: {data.get('current_load'):.2%}")
                
                results.append({
                    "node": url,
                    "metrics": data,
                    "success": True
                })
                
            except Exception as e:
                print(f"✗ {url}: Failed to get metrics - {str(e)}")
                results.append({
                    "node": url,
                    "success": False,
                    "error": str(e)
                })
        
        return results

    def test_latency_distribution(self, num_samples: int = 20) -> Dict:
        """Test latency distribution across different task types"""
        print(f"\n=== Testing Latency Distribution ({num_samples} samples per type) ===")
        results = []
        
        task_types = ["data_aggregation", "edge_analytics", "preprocessing", "caching"]
        
        for url in self.base_urls:
            print(f"\nTesting {url}...")
            node_latencies = {}
            
            for task_type in task_types:
                latencies = []
                
                for i in range(num_samples):
                    start = time.time()
                    success, data = self.submit_task(
                        url,
                        task_type,
                        {"test": f"sample_{i}"}
                    )
                    
                    if success:
                        task_id = data.get('id')
                        # Poll for completion
                        max_wait = 5
                        elapsed = 0
                        while elapsed < max_wait:
                            task_data = self.check_task_status(url, task_id)
                            if task_data.get('status') == 'completed':
                                break
                            time.sleep(0.1)
                            elapsed += 0.1
                        
                        latency = time.time() - start
                        latencies.append(latency)
                
                if latencies:
                    node_latencies[task_type] = {
                        "mean": statistics.mean(latencies),
                        "median": statistics.median(latencies),
                        "stdev": statistics.stdev(latencies) if len(latencies) > 1 else 0,
                        "min": min(latencies),
                        "max": max(latencies)
                    }
                    
                    print(f"  {task_type}:")
                    print(f"    Mean: {node_latencies[task_type]['mean']:.3f}s")
                    print(f"    Median: {node_latencies[task_type]['median']:.3f}s")
                    print(f"    Std Dev: {node_latencies[task_type]['stdev']:.3f}s")
            
            results.append({
                "node": url,
                "latencies": node_latencies
            })
        
        self.results["latency_tests"] = results
        return results

    def generate_report(self) -> str:
        """Generate a comprehensive test report"""
        report = f"""
{'='*80}
FOG COMPUTING TEST REPORT
Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}
{'='*80}

SUMMARY
-------
Nodes Tested: {len(self.base_urls)}
Test Categories: Health, Status, Task Submission, Load, Metrics, Latency

HEALTH CHECK RESULTS
--------------------
"""
        
        for hc in self.results.get("health_checks", []):
            status = "✓ PASS" if hc.get("status") else "✗ FAIL"
            report += f"{hc['node']}: {status}\n"
        
        report += "\nLOAD TEST RESULTS\n"
        report += "-" * 80 + "\n"
        
        for lt in self.results.get("load_tests", []):
            report += f"""
Node: {lt['node']}
  Total Tasks: {lt['total_tasks']}
  Successful: {lt['successful']}
  Failed: {lt['failed']}
  Throughput: {lt['throughput']:.2f} tasks/second
  Average Latency: {lt['avg_latency']:.3f}s
  P95 Latency: {lt['p95_latency']:.3f}s
"""
        
        return report

    def save_report(self, filename: str = "fog_test_report.txt"):
        """Save test report to file"""
        report = self.generate_report()
        with open(filename, 'w') as f:
            f.write(report)
        print(f"\n✓ Report saved to {filename}")


def main():
    """Main test execution"""
    # Configure fog node URLs (adjust ports if needed)
    fog_nodes = [
        "http://localhost:8081",
        "http://localhost:8082",
        "http://localhost:8083"
    ]
    
    print("="*80)
    print("FOG COMPUTING TEST SUITE")
    print("="*80)
    
    tester = FogComputeTester(fog_nodes)
    
    # Run all tests
    tester.test_health_check()
    tester.test_node_status()
    tester.test_task_submission()
    tester.test_metrics_collection()
    tester.test_concurrent_load(num_tasks=50)
    tester.test_latency_distribution(num_samples=10)
    
    # Generate and display report
    print(tester.generate_report())
    
    # Save results to file
    tester.save_report()
    
    # Save detailed results as JSON
    with open('fog_test_results.json', 'w') as f:
        json.dump(tester.results, f, indent=2, default=str)
    print("✓ Detailed results saved to fog_test_results.json")


if __name__ == "__main__":
    main()
