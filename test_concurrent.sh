#!/bin/bash

# 并发部署测试 - 3个服务器同时部署

CONFIG_FILE="$(dirname "$0")/test_concurrent.csv"
CLI_PATH="$(dirname "$0")/mailops"

echo "======================================"
echo "MailOps 并发部署测试"
echo "======================================"
echo ""
echo "测试配置:"
echo "  - 服务器数量: 3"
echo "  - 并发数: 3"
echo "  - 模式: dry_run"
echo ""
echo "开始测试..."
echo "======================================"
echo ""

# 创建 NDJSON 命令文件
cat > /tmp/test_concurrent_commands.jsonl << EOF
{"type":"START_RUN","config_path":"$CONFIG_FILE","concurrency":3,"dry_run":true}
EOF

# 运行 CLI 并提供 NDJSON 命令
cat /tmp/test_concurrent_commands.jsonl | $CLI_PATH --event-stream 2>&1 | tee /workspace/cli/output/concurrent_test.log

echo ""
echo "======================================"
echo "测试完成"
echo "======================================"
echo "详细日志: /workspace/cli/output/concurrent_test.log"