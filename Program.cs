using System;
using System.Windows.Forms;
using MailOpsDesktop.Views;

namespace MailOpsDesktop;

static class Program
{
    /// <summary>
    /// 應用程式的主進入點。
    /// </summary>
    [STAThread]
    static void Main()
    {
        Application.SetHighDpiMode(HighDpiMode.SystemAware);
        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);
        Application.Run(new MainForm());
    }
}