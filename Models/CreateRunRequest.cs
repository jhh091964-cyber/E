namespace MailOpsDesktop.Models
{
    public class CreateRunRequest
    {
        public string Domain { get; set; }
        public string Ip { get; set; }
        public bool DryRun { get; set; }
    }
}
