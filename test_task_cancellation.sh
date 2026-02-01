#!/bin/bash

# 任务取消功能测试

CONFIG_FILE="$(dirname "$0")/test_concurrent.csv"
CLI_PATH="$(dirname "$0")/mailops"

echo "======================================"
echo "MailOps 任务取消测试"
echo "======================================"
echo ""
echo "测试场景:"
echo "  1. 启动 3 个并发任务"
echo "  2. 等待 2 秒后发送 CANCEL_RUN 命令"
echo "  3. 验证任务被正确取消"
echo ""
echo "开始测试..."
echo "======================================"
echo ""

# 创建命名管道
PIPE_PATH="/tmp/mailops_cancel_pipe"
rm -f $PIPE_PATH
mkfifo $PIPE_PATH

# 启动 CLI 并读取命名管道
$CLI_PATH --event-stream < $PIPE_PATH 2>&1 | tee /workspace/cli/output/cancellation_test.log &
CLI_PID=$!

# 发送启动命令
echo '{"type":"START_RUN","config_path":"./test_concurrent.csv","concurrency":3,"dry_run":true}' > $PIPE_PATH

# 等待 2 秒让任务开始执行
sleep 2

echo ""
echo "发送 CANCEL_RUN 命令..."
echo "======================================"
echo ""

# 发送取消命令
echo '{"type":"CANCEL_RUN"}' > $PIPE_PATH

# 关闭管道以结束 CLI
exec 3>&-
sleep 1

# 清理
rm -f $PIPE_PATH

echo ""
echo "======================================"
echo "测试完成"
echo "======================================"
echo "详细日志: /workspace/cli/output/cancellation_test.log"