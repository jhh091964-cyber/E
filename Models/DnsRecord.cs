namespace MailOpsDesktop.Models
{
    public class DnsRecord
    {
        public string Type { get; set; }
        public string Name { get; set; }
        public int Ttl { get; set; }
        public string Value { get; set; }
    }
}
