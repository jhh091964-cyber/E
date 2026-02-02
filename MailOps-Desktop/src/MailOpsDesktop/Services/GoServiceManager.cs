using System.Diagnostics;
using MailOpsDesktop.Models;
using MailOpsDesktop.Services;

namespace MailOpsDesktop.Services;

public class GoServiceManager : IDisposable
{
    private Process? _serviceProcess;
    private readonly SecurityManager _securityManager;
    private readonly string _serviceExecutablePath;
    private readonly string _httpAddr;
    private bool _disposed = false;

    public event EventHandler<string>? OnLogReceived;
    public event EventHandler<ServiceState>? OnStatusChanged;

    public GoServiceManager(
        string serviceExecutablePath,
        string httpAddr,
        SecurityManager securityManager)
    {
        _serviceExecutablePath = serviceExecutablePath;
        _httpAddr = httpAddr;
        _securityManager = securityManager;
    }

    /// <summary>
    /// 啟動 Service
    /// </summary>
    public async Task<bool> StartAsync()
    {
        // 檢查是否已運行
        if (IsRunning())
        {
            Console.WriteLine("Service 已在運行");
            return true;
        }

        // 檢查執行檔是否存在
        if (!File.Exists(_serviceExecutablePath))
        {
            Console.WriteLine($"Service 執行檔不存在: {_serviceExecutablePath}");
            return false;
        }

        try
        {
            // 構建環境變數
            var environmentVariables = new Dictionary<string, string>
            {
                // 將 Token 注入環境變數（不使用命令列）
                { "CF_API_TOKEN", _securityManager.GetTokenForEnvironment() ?? "" },
                { "MAILOPS_HTTP_ADDR", _httpAddr }
            };

            // 構建進程啟動資訊
            var startInfo = new ProcessStartInfo
            {
                FileName = _serviceExecutablePath,
                Arguments = $"--http-addr={_httpAddr}",
                UseShellExecute = false,
                CreateNoWindow = true,  // 不顯示控制台視窗
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                WorkingDirectory = Path.GetDirectoryName(_serviceExecutablePath)
            };

            // 設置環境變數
            foreach (var kvp in environmentVariables)
            {
                startInfo.EnvironmentVariables[kvp.Key] = kvp.Value;
            }

            // 啟動進程
            _serviceProcess = Process.Start(startInfo);
            
            if (_serviceProcess == null)
            {
                Console.WriteLine("啟動 Service 失敗");
                return false;
            }

            // 設置進程事件處理
            _serviceProcess.EnableRaisingEvents = true;
            _serviceProcess.Exited += OnServiceExited;

            // 設置輸出捕獲
            _serviceProcess.OutputDataReceived += (sender, e) =>
            {
                if (!string.IsNullOrEmpty(e.Data))
                {
                    OnLogReceived?.Invoke(this, $"[INFO] {e.Data}");
                }
            };

            _serviceProcess.ErrorDataReceived += (sender, e) =>
            {
                if (!string.IsNullOrEmpty(e.Data))
                {
                    OnLogReceived?.Invoke(this, $"[ERROR] {e.Data}");
                }
            };

            _serviceProcess.BeginOutputReadLine();
            _serviceProcess.BeginErrorReadLine();

            Console.WriteLine($"Service 已啟動 (PID: {_serviceProcess.Id})");
            OnStatusChanged?.Invoke(this, ServiceState.Starting);

            return true;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"啟動 Service 時發生錯誤: {ex.Message}");
            return false;
        }
    }

    /// <summary>
    /// 停止 Service
    /// </summary>
    public async Task<bool> StopAsync()
    {
        if (_serviceProcess == null || _serviceProcess.HasExited)
        {
            Console.WriteLine("Service 未運行");
            return true;
        }

        try
        {
            Console.WriteLine("正在停止 Service...");
            
            // 優雅關閉
            _serviceProcess.CloseMainWindow();
            
            // 等待 5 秒
            if (!_serviceProcess.WaitForExit(5000))
            {
                Console.WriteLine("優雅關閉超時，強制終止進程");
                _serviceProcess.Kill(entireProcessTree: true);
            }

            Console.WriteLine("Service 已停止");
            OnStatusChanged?.Invoke(this, ServiceState.Stopped);
            return true;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"停止 Service 時發生錯誤: {ex.Message}");
            return false;
        }
    }

    /// <summary>
    /// 檢查 Service 是否在運行
    /// </summary>
    public bool IsRunning()
    {
        return _serviceProcess != null && !_serviceProcess.HasExited;
    }

    /// <summary>
    /// 獲取進程 ID
    /// </summary>
    public int? GetProcessId()
    {
        return _serviceProcess?.Id;
    }

    /// <summary>
    /// Service 退出事件處理
    /// </summary>
    private void OnServiceExited(object? sender, EventArgs e)
    {
        Console.WriteLine("Service 已退出");
        OnLogReceived?.Invoke(this, "[INFO] Service 已退出");
        OnStatusChanged?.Invoke(this, ServiceState.Stopped);
    }

    public void Dispose()
    {
        Dispose(true);
        GC.SuppressFinalize(this);
    }

    protected virtual void Dispose(bool disposing)
    {
        if (!_disposed)
        {
            if (disposing)
            {
                // 停止 Service
                if (IsRunning())
                {
                    StopAsync().Wait();
                }
                
                _serviceProcess?.Dispose();
            }
            _disposed = true;
        }
    }

    ~GoServiceManager()
    {
        Dispose(false);
    }
}