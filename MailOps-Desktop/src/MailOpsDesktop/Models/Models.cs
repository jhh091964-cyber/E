using System;
using System.Collections.Generic;
using System.Linq;

namespace MailOpsDesktop.Models;

/// <summary>
/// Service 狀態
/// </summary>
public class ServiceStatus
{
    public string Status { get; set; } = "unknown";
    public DateTime StartTime { get; set; }
    public int ActiveRuns { get; set; }
    public string HttpAddr { get; set; } = "";
}

/// <summary>
/// Service 狀態列舉
/// </summary>
public enum ServiceState
{
    Unknown,
    Starting,
    Running,
    Stopped,
    Error
}

/// <summary>
/// DNS 記錄
/// </summary>
public class DnsRecord
{
    public string Type { get; set; } = "";
    public string Name { get; set; } = "";
    public string Value { get; set; } = "";
    public int Priority { get; set; }
    public string Action { get; set; } = ""; // create, update, delete
}

/// <summary>
/// DNS 預覽結果
/// </summary>
public class DnsPreviewResult
{
    public string Domain { get; set; } = "";
    public List<DnsRecord> Records { get; set; } = new();
    public int CreateCount => Records.Count(r => r.Action == "create");
    public int UpdateCount => Records.Count(r => r.Action == "update");
    public int DeleteCount => Records.Count(r => r.Action == "delete");
}

/// <summary>
/// 建立 Run 請求
/// </summary>
public class CreateRunRequest
{
    public string ConfigFile { get; set; } = "";
    public Dictionary<string, object?> Params { get; set; } = new();
}

/// <summary>
/// 建立 Run 回應
/// </summary>
public class CreateRunResponse
{
    public string RunId { get; set; } = "";
    public string Status { get; set; } = "";
}

/// <summary>
/// 確認 Token 回應
/// </summary>
public class ConfirmResponse
{
    public string? ConfirmToken { get; set; }
    public string? Status { get; set; }
}

/// <summary>
/// 驗證 Token 回應
/// </summary>
public class ValidateResponse
{
    public bool Valid { get; set; }
    public string? Status { get; set; }
    public string? Message { get; set; }
}

/// <summary>
/// 執行 DNS 變更回應
/// </summary>
public class ExecuteResponse
{
    public bool Success { get; set; }
    public string? Status { get; set; }
    public string? Message { get; set; }
}