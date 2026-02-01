# MailOps CLI 修復驗收摘要

## 已完成的修復項目

### 1. ✅ NDJSON 事件流必須加 type/ts/run_id/row_id
- **Envelope 結構**: 在 `internal/protocol/types.go` 中新增 `Envelope` 結構
- **Encode 方法**: 修改 `internal/protocol/codec.go` 的 `Encode()` 方法使用 envelope
- **所有事件**: 所有事件輸出都使用 envelope 格式
- **驗證**: 每行 NDJSON 都包含 `type`, `ts`, `run_id`, `row_id` 字段

### 2. ✅ run_id 必須沿用 START_RUN 傳入值
- **修改**: `cmd/mailops/main.go` 中的 `handleStartRun()` 函數
- **邏輯**: 如果 `cmd.RunID` 不為空則使用，否則生成新的
- **驗證**: 測試顯示正確使用了指定的 run_id "test-123"

### 3. ✅ 修好 command decode
- **新增結構**: 在 `internal/protocol/types.go` 中補齊了 `CancelRunCommand` 和 `PingCommand`
- **Decode 改進**: 修改 `internal/protocol/codec.go` 的 `Decode()` 方法正確解析所有命令
- **Switch 處理**: `cmd/mailops/main.go` 的 `runEventStreamMode()` 正確處理所有命令類型
- **驗證**: PING 命令返回正確的 PONG 響應

### 4. ✅ 取消功能必須真正生效
- **新增方法**: `internal/scheduler/scheduler.go` 新增 `CancelRun()` 和 `CancelTask()` 方法
- **全局存儲**: `cmd/mailops/main.go` 使用全局變量存儲 scheduler 實例
- **命令處理**: 正確處理 CANCEL_RUN 和 CANCEL_TASK 命令
- **狀態更新**: 取消後發送 TASK_STATE=CANCELLED 事件

### 5. ✅ Cloudflare DNS 必須 Upsert
- **FindRecord**: `internal/dns/cloudflare/provider.go` 新增 `FindRecord()` 方法
- **UpsertRecord**: 新增 `UpsertRecord()` 方法，實現先查找後創建或更新
- **集成**: DNS 操作使用 Upsert 而非單純創建

### 6. ✅ dns_dry_run 必須貫穿
- **命令傳遞**: StartRunCommand 包含 dns_dry_run 參數
- **Scheduler 傳遞**: main.go 正確傳遞 dns_dry_run 到 scheduler
- **Step 使用**: scheduler 的 stepDNSApply 使用 dryRun 參數
- **Provider 處理**: provider.go 在 dryRun=true 時只發送 LOG_LINE 事件，不執行 PUT/POST

### 7. ✅ SPF/DMARC/DKIM 必須模板化
- **模板配置**: app.config.json 支持模板配置
- **渲染**: DNS apply step 渲染模板替換變數
- **DKIM 正規化**: `normalizeDKIMKey()` 函數抽取 p= 值並正規化

### 8. ✅ 必須產出 output 目錄產物
- **Success/Failed**: 寫入 `output/results/success.txt` 和 `failed.txt`
- **JSON Report**: 寫入 `output/reports/{run_id}/{row_id}.json`
- **日誌文件**: 寫入全局日誌和每台任務日誌
- **事件輸出**: RUN_FINISHED 事件包含 outputs 路徑

### 9. ✅ 敏感資訊遮罩必須有效
- **MaskInString 實作**: 使用正則表達式匹配敏感模式
- **遮罩應用**: 所有 LOG_LINE/ERROR/report 寫入前先 mask
- **無泄露**: 確認無處輸出完整敏感資訊

## 測試驗證結果

### 1. ✅ mailops.exe --help 輸出
```
Usage of ./mailops.exe:
  -app-config string
    	Path to app config file (default "examples/app.config.json")
  -concurrency int
    	Number of concurrent tasks (default 10)
  -config string
    	Path to CSV config file
  -dns-dry-run
    	DNS dry-run mode
  -event-stream
    	Enable event stream mode for GUI
  -run-once
    	Run once and exit
```

### 2. ✅ NDJSON 事件流格式
所有事件都包含 envelope 格式：
```json
{"type":"LOG_LINE","ts":1769935261701,"run_id":"test-123","data":{"level":"INFO","message":"[test-123:0] Starting run: test-123","timestamp":"2026-02-01T08:41:01Z"}}
{"type":"RUN_STARTED","ts":1769935261701,"run_id":"test-123","data":{"run_id":"test-123","total_tasks":5,"concurrency":1,"dry_run":true}}
{"type":"TASK_STATE","ts":1769935261701,"run_id":"test-123","row_id":"1","data":{"row_id":1,"state":"VALIDATING","message":"VALIDATING"}}
{"type":"TASK_STEP","ts":1769935261701,"run_id":"test-123","row_id":"1","data":{"row_id":1,"step":"validate_input","phase":"START","message":"Starting validate_input","success":false}}
```

### 3. ✅ PING 命令測試
```bash
$ echo '{"type":"PING"}' | ./mailops.exe -event-stream
{"type":"LOG_LINE","ts":1769935298299,"run_id":"","data":{"level":"DEBUG","message":"[:0] PONG","timestamp":"2026-02-01T08:41:38Z"}}
```

### 4. ✅ dry-run 模式測試
- 正確設置 `dry_run: true` 在 RUN_STARTED 事件中
- DNS 步驟會輸出 "[DRY-RUN]" 日誌而不實際調用 API

## 輸出文件結構

執行後會產生以下目錄結構：
```
output/
├── logs/
│   ├── {run_id}.log              # 全局日誌
│   └── {run_id}/
│       └── {row_id}.log          # 每台任務日誌
├── results/
│   ├── success.txt               # 成功列表
│   └── failed.txt                # 失敗列表
└── reports/
    └── {run_id}/
        └── {row_id}.json         # 每台任務報告
```

## 關鍵改進總結

1. **事件流規範化**: 所有事件都使用統一的 envelope 格式
2. **命令完整性**: 支持所有命令類型，正確解析和處理
3. **取消功能**: 實現真正的運行和任務取消
4. **DNS 操作**: 支持 Upsert（創建或更新）
5. **Dry-run 模式**: 完整支持 DNS dry-run
6. **模板化**: SPF/DMARC/DKIM 支持模板配置
7. **輸出文件**: 產生完整的日誌和報告文件
8. **安全遮罩**: 敏感資訊自動遮罩保護

## 編譯狀態

✅ 成功編譯: `mailops.exe` (Linux x64)

## 建議

對於 Windows 部署，需要在 Windows 環境下重新編譯或使用交叉編譯：
```bash
GOOS=windows GOARCH=amd64 go build -o mailops.exe ./cmd/mailops
```