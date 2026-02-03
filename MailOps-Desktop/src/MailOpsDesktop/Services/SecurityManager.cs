using System.Security;

namespace MailOpsDesktop.Services;

public class SecurityManager : IDisposable
{
    private SecureString? _token;
    private bool _disposed = false;

    /// <summary>
    /// 設置 Token（僅存於記憶體）
    /// </summary>
    public void SetToken(string token)
    {
        // 清除舊的 Token
        _token?.Dispose();
        
        // 創建新的 SecureString
        _token = new SecureString();
        foreach (char c in token)
        {
            _token.AppendChar(c);
        }
        _token.MakeReadOnly();
    }

    /// <summary>
    /// 獲取 Token（用於環境變數注入）
    /// </summary>
    public string? GetTokenForEnvironment()
    {
        if (_token == null) return null;
        
        // 將 SecureString 轉換為普通字符串（僅用於環境變數）
        IntPtr valuePtr = IntPtr.Zero;
        try
        {
            valuePtr = System.Runtime.InteropServices.Marshal.SecureStringToGlobalAllocUnicode(_token);
            return System.Runtime.InteropServices.Marshal.PtrToStringUni(valuePtr);
        }
        finally
        {
            System.Runtime.InteropServices.Marshal.ZeroFreeGlobalAllocUnicode(valuePtr);
        }
    }

    /// <summary>
    /// 獲取遮罩後的 Token（用於顯示）
    /// </summary>
    public string GetMaskedToken()
    {
        // 完全不顯示任何字元
        return "••••••••••••••••";
    }

    /// <summary>
    /// 清除 Token
    /// </summary>
    public void ClearToken()
    {
        _token?.Dispose();
        _token = null;
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
                ClearToken();
            }
            _disposed = true;
        }
    }

    ~SecurityManager()
    {
        Dispose(false);
    }
}