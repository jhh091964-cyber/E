using System.Text.Json;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.WinForms;
using MailOpsDesktop.Models;
using MailOpsDesktop.Services;

namespace MailOpsDesktop.Views;

public partial class MainForm : Form
{
    private WebView2 webView;
    private SecurityManager securityManager;
    private GoServiceManager? goServiceManager;
    private ApiService? apiService;
    
    private const string DefaultHttpAddr = "127.0.0.1:8080";

    public MainForm()
    {
        InitializeComponent();
        InitializeWebView();
        
        securityManager = new SecurityManager();
    }

    private void InitializeComponent()
    {
        this.Text = "MailOps 郵局搭建精靈";
        this.Size = new Size(900, 700);
        this.StartPosition = FormStartPosition.CenterScreen;
        this.FormBorderStyle = FormBorderStyle.FixedSingle;
        this.MaximizeBox = false;
    }

    private async void InitializeWebView()
    {
        webView = new WebView2();
        webView.Dock = DockStyle.Fill;
        this.Controls.Add(webView);

        var env = await CoreWebView2Environment.CreateAsync();
        await webView.EnsureCoreWebView2Async(env);
        
        // Enable web message receiving
        webView.CoreWebView2.WebMessageReceived += OnWebMessageReceived;
        
        // Set dev tools (disable in production)
        webView.CoreWebView2.Settings.AreDevToolsEnabled = false;
        
        // Load local HTML
        string webPath = Path.Combine(Application.StartupPath, "Web", "index.html");
        if (File.Exists(webPath))
        {
            webView.Source = new Uri($"file:///{webPath.Replace('\\', '/')}");
        }
        else
        {
            MessageBox.Show($"找不到 Web UI 文件：{webPath}", "錯誤", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
    }

    private async void OnWebMessageReceived(object? sender, CoreWebView2WebMessageReceivedEventArgs e)
    {
        try
        {
            string messageJson = e.TryGetWebMessageAsString();
            Console.WriteLine($"Received from WebView: {messageJson}");
            
            var message = JsonSerializer.Deserialize<WebMessage>(messageJson);
            
            if (message == null) return;
            
            switch (message.Type)
            {
                case "setToken":
                    await HandleSetToken(message);
                    break;
                    
                case "restartService":
                    await HandleRestartService(message);
                    break;
                    
                case "checkEnvironment":
                    await HandleCheckEnvironment(message);
                    break;
                    
                case "loadDnsPreview":
                    await HandleLoadDnsPreview(message);
                    break;
                    
                case "executeDnsChanges":
                    await HandleExecuteDnsChanges(message);
                    break;
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine($"處理 Web 消息時發生錯誤: {ex.Message}");
            SendErrorToWebView(ex.Message);
        }
    }

    private async Task HandleSetToken(WebMessage message)
    {
        try
        {
            var requestData = JsonSerializer.Deserialize<Dictionary<string, string>>(JsonSerializer.Serialize(message.Data));
            if (requestData == null || !requestData.ContainsKey("token"))
            {
                throw new ArgumentException("Invalid token data");
            }
            
            // Store token securely in memory
            securityManager.SetToken(requestData["token"]);
            Console.WriteLine("Token 已安全存儲到記憶體");
            
            // Success response
            SendToWebView(new WebMessage
            {
                Type = "tokenSetResult",
                Data = new { success = true }
            });
        }
        catch (Exception ex)
        {
            SendToWebView(new WebMessage
            {
                Type = "tokenSetResult",
                Data = new { success = false, error = ex.Message }
            });
        }
    }

    private async Task HandleRestartService(WebMessage message)
    {
        try
        {
            // Stop existing service if running
            if (goServiceManager != null && goServiceManager.IsRunning())
            {
                Console.WriteLine("停止現有 Service...");
                await goServiceManager.StopAsync();
                goServiceManager.Dispose();
                goServiceManager = null;
            }
            
            // Start new service with token
            await StartServiceAsync();
            
            Console.WriteLine("Service 重新啟動成功");
            
            // Success response
            SendToWebView(new WebMessage
            {
                Type = "restartServiceResult",
                Data = new { success = true }
            });
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Service 重新啟動失敗: {ex.Message}");
            SendToWebView(new WebMessage
            {
                Type = "restartServiceResult",
                Data = new { success = false, error = ex.Message }
            });
        }
    }

    private async Task HandleCheckEnvironment(WebMessage message)
    {
        try
        {
            // Ensure service is running
            if (apiService == null)
            {
                await StartServiceAsync();
            }
            
            // Check service status
            var status = await apiService!.GetStatusAsync();
            bool serviceSuccess = status != null && status.Status == "running";
            
            // Try to validate token by creating a test run
            bool tokenSuccess = true;
            string tokenMessage = "已驗證";
            
            try
            {
                var requestData = JsonSerializer.Deserialize<Dictionary<string, string>>(JsonSerializer.Serialize(message.Data));
                if (requestData != null)
                {
                    var testResponse = await apiService.CreateRunAsync(new CreateRunRequest
                    {
                        ConfigFile = "",
                        Params = new Dictionary<string, object>
                        {
                            { "domain", requestData.GetValueOrDefault("domain", "") },
                            { "vps_ip", requestData.GetValueOrDefault("vpsIp", "") },
                            { "profile", "cloudflare" },
                            { "dry_run", true }
                        }
                    });
                    
                    tokenSuccess = testResponse != null;
                    tokenMessage = tokenSuccess ? "已驗證" : "驗證失敗";
                }
            }
            catch
            {
                tokenSuccess = false;
                tokenMessage = "Token 無效或權限不足";
            }
            
            // Validate domain format
            bool domainSuccess = true;
            string domainMessage = "已驗證";
            
            try
            {
                var requestData = JsonSerializer.Deserialize<Dictionary<string, string>>(JsonSerializer.Serialize(message.Data));
                if (requestData != null)
                {
                    string domain = requestData.GetValueOrDefault("domain", "");
                    var domainRegex = new System.Text.RegularExpressions.Regex(@"^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9](?:\.[a-zA-Z]{2,})+$");
                    domainSuccess = domainRegex.IsMatch(domain);
                    domainMessage = domainSuccess ? "已驗證" : "格式錯誤";
                }
            }
            catch
            {
                domainSuccess = false;
                domainMessage = "驗證失敗";
            }
            
            SendToWebView(new WebMessage
            {
                Type = "environmentCheckResult",
                Data = new { 
                    serviceSuccess = serviceSuccess,
                    serviceMessage = serviceSuccess ? "正常運行" : "未運行",
                    tokenSuccess = tokenSuccess,
                    tokenMessage = tokenMessage,
                    domainSuccess = domainSuccess,
                    domainMessage = domainMessage
                }
            });
        }
        catch (Exception ex)
        {
            Console.WriteLine($"環境檢查失敗: {ex.Message}");
            SendToWebView(new WebMessage
            {
                Type = "environmentCheckResult",
                Data = new { 
                    serviceSuccess = false, 
                    serviceMessage = "檢查失敗",
                    tokenSuccess = false,
                    tokenMessage = "檢查失敗",
                    domainSuccess = false,
                    domainMessage = "檢查失敗",
                    error = ex.Message
                }
            });
        }
    }

    private async Task HandleLoadDnsPreview(WebMessage message)
    {
        try
        {
            var requestData = JsonSerializer.Deserialize<Dictionary<string, string>>(JsonSerializer.Serialize(message.Data));
            if (requestData == null)
            {
                throw new ArgumentException("Invalid request data");
            }
            
            string domain = requestData.GetValueOrDefault("domain", "");
            string vpsIp = requestData.GetValueOrDefault("vpsIp", "");
            
            // Create a dry-run run
            var createResponse = await apiService!.CreateRunAsync(new CreateRunRequest
            {
                ConfigFile = "",
                Params = new Dictionary<string, object>
                {
                    { "domain", domain },
                    { "vps_ip", vpsIp },
                    { "profile", "cloudflare" },
                    { "dry_run", true }
                }
            });
            
            if (createResponse == null || string.IsNullOrEmpty(createResponse.RunId))
            {
                throw new Exception("建立作業失敗");
            }
            
            // Get DNS preview
            var preview = await apiService.GetDnsPreviewAsync(createResponse.RunId);
            
            if (preview == null)
            {
                throw new Exception("取得 DNS 預覽失敗");
            }
            
            SendToWebView(new WebMessage
            {
                Type = "dnsPreviewResult",
                Data = new { success = true, data = preview }
            });
        }
        catch (Exception ex)
        {
            Console.WriteLine($"載入 DNS 預覽失敗: {ex.Message}");
            SendToWebView(new WebMessage
            {
                Type = "dnsPreviewResult",
                Data = new { success = false, error = ex.Message }
            });
        }
    }

    private async Task HandleExecuteDnsChanges(WebMessage message)
    {
        try
        {
            var requestData = JsonSerializer.Deserialize<Dictionary<string, string>>(JsonSerializer.Serialize(message.Data));
            if (requestData == null)
            {
                throw new ArgumentException("Invalid request data");
            }
            
            string domain = requestData.GetValueOrDefault("domain", "");
            string vpsIp = requestData.GetValueOrDefault("vpsIp", "");
            
            // Step 1: Create dry-run run for preview
            UpdateExecuteProgress("preview", "running");
            var createResponse = await apiService!.CreateRunAsync(new CreateRunRequest
            {
                ConfigFile = "",
                Params = new Dictionary<string, object>
                {
                    { "domain", domain },
                    { "vps_ip", vpsIp },
                    { "profile", "cloudflare" },
                    { "dry_run", true }
                }
            });
            
            if (createResponse == null || string.IsNullOrEmpty(createResponse.RunId))
            {
                throw new Exception("建立作業失敗");
            }
            
            UpdateExecuteProgress("preview", "completed");
            UpdateProgressBar(25);
            
            // Step 2: Generate confirm token
            UpdateExecuteProgress("confirm", "running");
            var confirmResponse = await apiService.ConfirmAsync(createResponse.RunId);
            
            if (confirmResponse == null || string.IsNullOrEmpty(confirmResponse.ConfirmToken))
            {
                throw new Exception("生成安全碼失敗");
            }
            
            UpdateExecuteProgress("confirm", "completed");
            UpdateProgressBar(50);
            
            // Step 3: Validate confirm token
            UpdateExecuteProgress("validate", "running");
            var validateResponse = await apiService.ValidateAsync(createResponse.RunId, confirmResponse.ConfirmToken);
            
            if (validateResponse == null || !validateResponse.Valid)
            {
                throw new Exception("驗證安全碼失敗");
            }
            
            UpdateExecuteProgress("validate", "completed");
            UpdateProgressBar(75);
            
            // Step 4: Execute DNS changes
            UpdateExecuteProgress("execute", "running");
            var executeResponse = await apiService.ExecuteAsync(createResponse.RunId, confirmResponse.ConfirmToken);
            
            if (executeResponse == null || !executeResponse.Success)
            {
                throw new Exception("執行 DNS 變更失敗");
            }
            
            UpdateExecuteProgress("execute", "completed");
            UpdateProgressBar(100);
            
            // Success
            SendToWebView(new WebMessage
            {
                Type = "executeComplete",
                Data = new { success = true }
            });
        }
        catch (Exception ex)
        {
            Console.WriteLine($"執行 DNS 變更失敗: {ex.Message}");
            SendToWebView(new WebMessage
            {
                Type = "executeComplete",
                Data = new { success = false, error = ex.Message }
            });
        }
    }

    private void UpdateExecuteProgress(string step, string status)
    {
        SendToWebView(new WebMessage
        {
            Type = "executeProgress",
            Data = new { step = step, status = status }
        });
    }

    private void UpdateProgressBar(int percent)
    {
        SendToWebView(new WebMessage
        {
            Type = "executeProgress",
            Data = new { percent = percent }
        });
    }

    private void SendErrorToWebView(string error)
    {
        SendToWebView(new WebMessage
        {
            Type = "error",
            Data = new { error = error }
        });
    }

    private async Task StartServiceAsync()
    {
        // Get service executable path
        string servicePath = Path.Combine(Application.StartupPath, "mailops-service.exe");
        
        if (!File.Exists(servicePath))
        {
            throw new FileNotFoundException($"找不到 Go Service 執行檔：{servicePath}");
        }
        
        // Initialize service manager
        goServiceManager = new GoServiceManager(servicePath, DefaultHttpAddr, securityManager);
        goServiceManager.OnLogReceived += OnServiceLogReceived;
        goServiceManager.OnStatusChanged += OnServiceStatusChanged;
        
        // Start service
        bool success = await goServiceManager.StartAsync();
        if (!success)
        {
            throw new Exception("啟動服務失敗");
        }
        
        // Initialize API service
        apiService = new ApiService($"http://{DefaultHttpAddr}", securityManager);
        
        // Wait for service to be ready
        bool ready = await apiService.WaitForServiceReadyAsync();
        if (!ready)
        {
            throw new Exception("服務啟動超時");
        }
        
        Console.WriteLine("Service 啟動成功");
    }

    private void OnServiceLogReceived(object? sender, string log)
    {
        Console.WriteLine(log);
    }

    private void OnServiceStatusChanged(object? sender, ServiceState status)
    {
        Console.WriteLine($"Service 狀態變更: {status}");
    }

    private void SendToWebView(WebMessage message)
    {
        if (webView?.CoreWebView2 != null)
        {
            string json = JsonSerializer.Serialize(message);
            webView.CoreWebView2.PostWebMessageAsJson(json);
            Console.WriteLine($"Sent to WebView: {json}");
        }
    }

    protected override void OnFormClosing(FormClosingEventArgs e)
    {
        // Stop service
        if (goServiceManager != null && goServiceManager.IsRunning())
        {
            goServiceManager.StopAsync().Wait();
            goServiceManager.Dispose();
        }
        
        // Clear token
        securityManager.ClearToken();
        securityManager.Dispose();
        
        base.OnFormClosing(e);
    }
}

// WebMessage structure for WebView2 communication
public class WebMessage
{
    public string Type { get; set; } = "";
    public object? Data { get; set; }
}