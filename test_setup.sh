#!/bin/bash

# MailOps 真实测试快速设置脚本
# 使用方法: bash test_setup.sh

set -e

echo "=========================================="
echo "  MailOps 真实功能测试设置向导"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查是否提供了测试配置
echo -e "${YELLOW}📋 检查测试配置文件...${NC}"

if [ ! -f "my_test_servers.csv" ]; then
    echo -e "${RED}❌ 未找到 my_test_servers.csv 文件${NC}"
    echo ""
    echo "请先按照以下步骤创建配置文件："
    echo ""
    echo "1. 复制模板："
    echo "   cp test_servers.csv my_test_servers.csv"
    echo ""
    echo "2. 编辑配置文件，填入真实信息："
    echo "   nano my_test_servers.csv"
    echo ""
    echo "3. 必需字段："
    echo "   - cf_api_token: Cloudflare API Token"
    echo "   - cf_zone: Cloudflare 域名（如 example.com）"
    echo "   - server_ip: 服务器 IP 地址"
    echo "   - server_port: SSH 端口（默认 22）"
    echo "   - server_user: SSH 用户名（如 root）"
    echo "   - server_password: SSH 密码"
    echo "   - host: 邮件主机名（如 mail）"
    echo "   - domain: 完整域名（如 mail1.example.com）"
    echo "   - deploy_profile: postfix_dovecot 或 docker_mailserver"
    echo ""
    exit 1
fi

echo -e "${GREEN}✅ 找到配置文件: my_test_servers.csv${NC}"
echo ""

# 验证配置文件
echo -e "${YELLOW}🔍 验证配置文件格式...${NC}"

# 检查必需字段
required_fields="row_id,cf_api_token,cf_zone,server_ip,server_port,server_user,server_password,host,domain,deploy_profile"
header=$(head -n 1 my_test_servers.csv)

for field in ${required_fields//,/ }; do
    if [[ ! $header =~ $field ]]; then
        echo -e "${RED}❌ 缺少必需字段: $field${NC}"
        exit 1
    fi
done

echo -e "${GREEN}✅ 配置文件格式正确${NC}"
echo ""

# 显示配置摘要
echo -e "${YELLOW}📊 配置摘要:${NC}"
echo "-----------------------------------"
server_count=$(tail -n +2 my_test_servers.csv | wc -l)
echo "服务器数量: $server_count"
echo "配置文件: my_test_servers.csv"
echo "-----------------------------------"
echo ""

# 测试模式选择
echo -e "${YELLOW}🎯 请选择测试模式:${NC}"
echo ""
echo "1) 干运行测试（推荐）- 不实际创建 DNS 记录，不安装软件"
echo "2) 真实部署 - 实际部署邮件服务器和配置 DNS"
echo "3) 并发测试 - 测试多服务器并发部署"
echo ""
read -p "请输入选项 [1-3]: " mode

case $mode in
    1)
        echo ""
        echo -e "${GREEN}🧪 选择: 干运行测试${NC}"
        dry_run="true"
        concurrency="1"
        ;;
    2)
        echo ""
        echo -e "${YELLOW}⚠️  选择: 真实部署${NC}"
        echo -e "${RED}警告: 此操作将：${NC}"
        echo "  - 连接到您的服务器"
        echo "  - 安装邮件服务器软件"
        echo "  - 创建 Cloudflare DNS 记录"
        echo ""
        read -p "确认继续? (yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            echo "操作已取消"
            exit 0
        fi
        dry_run="false"
        concurrency="1"
        ;;
    3)
        echo ""
        echo -e "${GREEN}🚀 选择: 并发测试${NC}"
        dry_run="false"
        read -p "并发数量 [1-10]: " concurrency
        concurrency=${concurrency:-2}
        ;;
    *)
        echo -e "${RED}❌ 无效选项${NC}"
        exit 1
        ;;
esac

echo ""
echo -e "${YELLOW}📋 测试配置:${NC}"
echo "  模式: $([ "$dry_run" = "true" ] && echo "干运行" || echo "真实部署")"
echo "  并发数: $concurrency"
echo "  配置文件: my_test_servers.csv"
echo ""

# 创建日志目录
mkdir -p ../gui/output/logs
mkdir -p ../gui/output/results
mkdir -p ../gui/output/reports

# 执行测试
echo -e "${YELLOW}🚀 开始测试...${NC}"
echo ""
echo "=========================================="
echo "  测试输出日志"
echo "=========================================="
echo ""

# 构建 START_RUN 命令
cmd=$(cat <<EOF
{"type":"START_RUN","config_path":"my_test_servers.csv","concurrency":$concurrency,"dry_run":$dry_run}
EOF
)

# 执行命令并保存日志
timestamp=$(date +%Y%m%d_%H%M%S)
log_file="../gui/output/logs/test_${timestamp}.log"

echo "$cmd" | ./mailops --event-stream 2>&1 | tee "$log_file"

echo ""
echo "=========================================="
echo "  测试完成"
echo "=========================================="
echo ""
echo -e "${GREEN}✅ 日志已保存到: $log_file${NC}"
echo ""
echo -e "${YELLOW}📊 查看详细结果:${NC}"
echo "  - 日志文件: $log_file"
echo "  - 结果文件: ../gui/output/results/"
echo "  - 报告文件: ../gui/output/reports/"
echo ""

if [ "$dry_run" = "true" ]; then
    echo -e "${GREEN}🎉 干运行测试完成！${NC}"
    echo "如果一切正常，可以进行真实部署。"
else
    echo -e "${GREEN}🎉 部署完成！${NC}"
    echo "请验证："
    echo "  1. 服务器上邮件服务是否运行"
    echo "  2. Cloudflare DNS 记录是否创建"
    echo "  3. 端口是否正常监听"
fi

echo ""
echo -e "${YELLOW}📖 查看完整测试指南:${NC}"
echo "  cat REAL_TESTING_GUIDE.md"
echo ""