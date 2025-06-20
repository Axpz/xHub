#!/bin/bash

echo "=== xHub Debug Test Script ==="
echo ""

# 检查服务器是否运行
echo "1. 检查服务器状态..."
curl -s http://localhost:20080/health | jq .

echo ""
echo "2. 查看当前调试信息..."
curl -s http://localhost:20080/api/v1/debug | jq .

echo ""
echo "3. 创建测试任务..."
curl -s -X POST http://localhost:20080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "test",
    "command": "echo hello world",
    "parameters": {},
    "timeout_seconds": 30
  }' | jq .

echo ""
echo "4. 再次查看调试信息..."
sleep 2
curl -s http://localhost:20080/api/v1/debug | jq .

echo ""
echo "5. 查看待处理任务..."
curl -s "http://localhost:20080/api/v1/tasks?status=pending" | jq .

echo ""
echo "=== 测试完成 ==="
echo "如果agent在线，应该能在日志中看到任务分配信息" 