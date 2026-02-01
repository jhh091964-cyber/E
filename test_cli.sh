#!/bin/bash

# MailOps CLI 测试脚本
# 使用 NDJSON 协议与 CLI 通信

CONFIG_FILE="$(dirname "$0")/my_test_servers.csv"
CLI_PATH="$(dirname "$0")/mailops"

echo "=== MailOps CLI 真实功能测试 ==="
echo ""
echo "配置文件: $CONFIG_FILE"
echo ""

# 创建 NDJSON 命令文件
cat > /tmp/test_commands.jsonl << EOF
{"type":"START_RUN","config_path":"$CONFIG_FILE","concurrency":1,"dry_run":true}
EOF

echo "发送 START_RUN 命令..."
echo ""

# 运行 CLI 并提供 NDJSON 命令
cat /tmp/test_commands.jsonl | $CLI_PATH --event-stream --dns-dry-run 2>&1 | tee /workspace/cli/output/real_test.log

echo ""
echo "=== 测试完成 ==="
echo "日志已保存到: /workspace/cli/output/real_test.log"