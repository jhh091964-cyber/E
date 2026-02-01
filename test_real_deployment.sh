#!/bin/bash

# MailOps CLI 完整真实部署测试
# 包含：SSH连接、包安装、DNS记录创建等真实操作

CONFIG_FILE="$(dirname "$0")/my_test_servers.csv"
CLI_PATH="$(dirname "$0")/mailops"

echo "======================================"
echo "MailOps CLI 完整真实部署测试"
echo "======================================"
echo ""
echo "⚠️  警告：这将执行以下真实操作："
echo "  - SSH 连接到远程服务器"
echo "  - 安装邮件服务器软件包"
echo "  - 生成 DKIM 密钥"
echo "  - 在 Cloudflare 创建 DNS 记录"
echo ""
echo "配置文件: $CONFIG_FILE"
echo "目标服务器:"
cat "$CONFIG_FILE" | tail -1 | awk -F',' '{print "  IP: " $4 "\n  域名: " $10}'
echo ""
# 自动确认真实部署
confirm="yes"

echo ""
echo "开始真实部署..."
echo "======================================"
echo ""

# 创建 NDJSON 命令文件（非 dry-run）
cat > /tmp/test_real_commands.jsonl << EOF
{"type":"START_RUN","config_path":"$CONFIG_FILE","concurrency":1,"dry_run":false}
EOF

# 运行 CLI 并提供 NDJSON 命令
cat /tmp/test_real_commands.jsonl | $CLI_PATH --event-stream 2>&1 | tee /workspace/cli/output/real_deployment.log

echo ""
echo "======================================"
echo "部署完成"
echo "======================================"
echo "详细日志: /workspace/cli/output/real_deployment.log"