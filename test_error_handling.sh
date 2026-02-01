#!/bin/bash

# 错误处理和重试机制测试

CONFIG_FILE="$(dirname "$0")/test_error_scenarios.csv"
CLI_PATH="$(dirname "$0")/mailops"

echo "======================================"
echo "MailOps 错误处理测试"
echo "======================================"
echo ""
echo "测试场景:"
echo "  1. 无效的 Cloudflare API Token"
echo "  2. 无效的服务器 IP 地址"
echo "  3. SSH 认证失败"
echo ""
echo "开始测试..."
echo "======================================"
echo ""

# 创建 NDJSON 命令文件
cat > /tmp/test_error_commands.jsonl << EOF
{"type":"START_RUN","config_path":"$CONFIG_FILE","concurrency":3,"dry_run":false}
EOF

# 运行 CLI 并提供 NDJSON 命令
cat /tmp/test_error_commands.jsonl | $CLI_PATH --event-stream 2>&1 | tee /workspace/cli/output/error_test.log

echo ""
echo "======================================"
echo "测试完成"
echo "======================================"
echo "详细日志: /workspace/cli/output/error_test.log"