namespace MailOpsDesktop.Models
{
    public class ConfirmResponse
    {
        public string ConfirmId { get; set; }
        public string MaskedToken { get; set; }
        public int ExpiresIn { get; set; }
    }
}
