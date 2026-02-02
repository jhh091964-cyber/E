using System.Text;
using System.Text.Json;
using MailOpsDesktop.Models;

namespace MailOpsDesktop.Services;

public class ApiService
{
    private readonly HttpClient _httpClient;
    private readonly string _baseUrl;
    private readonly SecurityManager _securityManager;

    public ApiService(string baseUrl, SecurityManager securityManager)
    {
        _baseUrl = baseUrl.EndsWith("/") ? baseUrl.TrimEnd('/') : baseUrl;
        _securityManager = securityManager;
        _httpClient = new HttpClient
        {
            Timeout = TimeSpan.FromSeconds(30)
        };
    }

    /// <summary>
    /// 獲取 Service 狀態
    /// </summary>
    public async Task<ServiceStatus?> GetStatusAsync()
    {
        try
        {
            var response = await _httpClient.GetAsync($"{_baseUrl}/api/status");
            response.EnsureSuccessStatusCode();
            
            var json = await response.Content.ReadAsStringAsync();
            return JsonSerializer.Deserialize<ServiceStatus>(json);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"獲取狀態失敗: {ex.Message}");
            return null;
        }
    }

    /// <summary>
    /// 建立 Run
    /// </summary>
    public async Task<CreateRunResponse?> CreateRunAsync(CreateRunRequest request)
    {
        try
        {
            var json = JsonSerializer.Serialize(request);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            
            var response = await _httpClient.PostAsync($"{_baseUrl}/api/runs", content);
            response.EnsureSuccessStatusCode();
            
            var responseJson = await response.Content.ReadAsStringAsync();
            return JsonSerializer.Deserialize<CreateRunResponse>(responseJson);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"建立 Run 失敗: {ex.Message}");
            return null;
        }
    }

    /// <summary>
    /// 獲取 DNS 預覽
    /// </summary>
    public async Task<DnsPreviewResult?> GetDnsPreviewAsync(string runId)
    {
        try
        {
            var response = await _httpClient.GetAsync($"{_baseUrl}/api/dns/preview?run_id={runId}");
            response.EnsureSuccessStatusCode();
            
            var json = await response.Content.ReadAsStringAsync();
            return JsonSerializer.Deserialize<DnsPreviewResult>(json);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"獲取 DNS 預覽失敗: {ex.Message}");
            return null;
        }
    }

    /// <summary>
    /// 等待 Service 就緒
    /// </summary>
    public async Task<bool> WaitForServiceReadyAsync(int timeoutMs = 10000)
    {
        var startTime = DateTime.Now;
        
        while (DateTime.Now - startTime < TimeSpan.FromMilliseconds(timeoutMs))
        {
            try
            {
                var status = await GetStatusAsync();
                if (status != null && status.Status == "running")
                {
                    return true;
                }
            }
            catch
            {
                // 繼續等待
            }
            
            await Task.Delay(500);
        }
        
        return false;
    }

    /// <summary>
    /// 生成確認 Token (Confirm)
    /// </summary>
    public async Task<ConfirmResponse?> ConfirmAsync(string runId)
    {
        try
        {
            var request = new { run_id = runId };
            var json = JsonSerializer.Serialize(request);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            
            var response = await _httpClient.PostAsync($"{_baseUrl}/api/dns/confirm", content);
            response.EnsureSuccessStatusCode();
            
            var responseJson = await response.Content.ReadAsStringAsync();
            return JsonSerializer.Deserialize<ConfirmResponse>(responseJson);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"生成確認 Token 失敗: {ex.Message}");
            return null;
        }
    }

    /// <summary>
    /// 驗證確認 Token (Validate)
    /// </summary>
    public async Task<ValidateResponse?> ValidateAsync(string runId, string confirmToken)
    {
        try
        {
            var request = new 
            { 
                run_id = runId,
                confirm_token = confirmToken
            };
            var json = JsonSerializer.Serialize(request);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            
            var response = await _httpClient.PostAsync($"{_baseUrl}/api/dns/validate", content);
            response.EnsureSuccessStatusCode();
            
            var responseJson = await response.Content.ReadAsStringAsync();
            return JsonSerializer.Deserialize<ValidateResponse>(responseJson);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"驗證確認 Token 失敗: {ex.Message}");
            return null;
        }
    }

    /// <summary>
    /// 執行 DNS 變更 (Execute)
    /// </summary>
    public async Task<ExecuteResponse?> ExecuteAsync(string runId, string confirmToken)
    {
        try
        {
            var request = new 
            { 
                run_id = runId,
                confirm_token = confirmToken
            };
            var json = JsonSerializer.Serialize(request);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            
            var response = await _httpClient.PostAsync($"{_baseUrl}/api/dns/execute", content);
            response.EnsureSuccessStatusCode();
            
            var responseJson = await response.Content.ReadAsStringAsync();
            return JsonSerializer.Deserialize<ExecuteResponse>(responseJson);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"執行 DNS 變更失敗: {ex.Message}");
            return null;
        }
    }

    public void Dispose()
    {
        _httpClient?.Dispose();
    }
}