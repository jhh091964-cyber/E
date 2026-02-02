// Models/Models.cs
namespace MailOpsDesktop.Models
{
    public enum ServiceState { Unknown, Starting, Running, Stopped, Error }

    public class ServiceStatus
    {
        public string status { get; set; } = "";
        public string version { get; set; } = "";
    }

    public class CreateRunRequest
    {
        public string config_file { get; set; } = "";
        public object? @params { get; set; }
    }

    public class CreateRunResponse
    {
        public string id { get; set; } = "";
    }

    public class DnsPreviewResult
    {
        public int total { get; set; }
        public object[] records { get; set; } = System.Array.Empty<object>();
    }

    public class ConfirmResponse
    {
        public string confirm_id { get; set; } = "";
        public string masked_token { get; set; } = "";
        public int expires_in_sec { get; set; }
    }

    public class ValidateResponse
    {
        public string status { get; set; } = "";
    }

    public class ExecuteResponse
    {
        public string status { get; set; } = "";
        public string message { get; set; } = "";
        public int records_written { get; set; }
    }
}
