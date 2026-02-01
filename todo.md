# MailOps CLI 修復任務清單

## 已完成修復項目 (1-9)
- [x] 1) NDJSON 事件流必須加 type/ts/run_id/row_id
- [x] 2) run_id 必須沿用 START_RUN 傳入值
- [x] 3) 修好 command decode
- [x] 4) 取消功能必須真正生效
- [x] 5) Cloudflare DNS 必須 Upsert
- [x] 6) dns_dry_run 必須貫穿
- [x] 7) SPF/DMARC/DKIM 必須模板化
- [x] 8) 必須產出 output 目錄產物
- [x] 9) 敏感資訊遮罩必須有效

## 待驗收項目 (10)
- [ ] 1) mailops.exe --help 輸出
- [ ] 2) dry-run 執行測試
- [ ] 3) output 目錄結構
- [ ] 4) NDJSON 事件流片段
- [ ] 5) CancelRun/CancelTask 測試
