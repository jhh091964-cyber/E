namespace MailOpsDesktop.Models
{
    public class ServiceStatus
    {
        public string Status { get; set; }
    }

    public class CreateRunRequest
    {
        public bool DryRun { get; set; }
    }

    public class CreateRunResponse
    {
        public string RunId { get; set; }
    }

    public class DnsPreviewResult
    {
        public string Type { get; set; }
        public string Name { get; set; }
        public int Ttl { get; set; }
        public string Value { get; set; }
    }

    public class ConfirmResponse
    {
        public string ConfirmId { get; set; }
        public string MaskedToken { get; set; }
        public int ExpiresInSec { get; set; }
    }

    public class ValidateResponse
    {
        public bool Valid { get; set; }
    }
}
