package profiles

import (
	"fmt"
	"mailops/internal/ssh"
	"strings"
	"time"
)

// PostfixDovecotProfile implements Postfix + Dovecot deployment
type PostfixDovecotProfile struct {
	Domain       string
	Hostname     string
	DKIMSelector string
	DKIMKeySize  int
}

// DeployResult represents deployment result
type DeployResult struct {
	Version string
	Message string
}

// Deploy deploys Postfix + Dovecot mail server
func (p *PostfixDovecotProfile) Deploy(client *ssh.Client) (*DeployResult, error) {
	// Step 1: Install mail packages
	packages := []string{
		"postfix",
		"dovecot-core",
		"dovecot-imapd",
		"dovecot-pop3d",
		"opendkim",
		"opendkim-tools",
		"mailutils",
	}
	
	for _, pkg := range packages {
		if err := client.InstallPackage(pkg); err != nil {
			return nil, fmt.Errorf("failed to install %s: %v", pkg, err)
		}
	}
	
	// Step 2: Configure Postfix
	if err := p.configurePostfix(client); err != nil {
		return nil, fmt.Errorf("failed to configure Postfix: %v", err)
	}
	
	// Step 3: Configure Dovecot
	if err := p.configureDovecot(client); err != nil {
		return nil, fmt.Errorf("failed to configure Dovecot: %v", err)
	}
	
	// Step 4: Configure OpenDKIM
	if err := p.configureOpenDKIM(client); err != nil {
		return nil, fmt.Errorf("failed to configure OpenDKIM: %v", err)
	}
	
	// Step 5: Start services
	services := []string{"postfix", "dovecot"}
	for _, svc := range services {
		_, err := client.ExecuteCommandWithOutput(fmt.Sprintf("systemctl enable %s", svc), 30*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to enable %s: %v", svc, err)
		}
		
		_, err = client.ExecuteCommandWithOutput(fmt.Sprintf("systemctl restart %s", svc), 30*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to restart %s: %v", svc, err)
		}
	}
	
	return &DeployResult{
		Version: "Postfix + Dovecot",
		Message: "Deployment successful",
	}, nil
}

// configurePostfix configures Postfix
func (p *PostfixDovecotProfile) configurePostfix(client *ssh.Client) error {
	// Configure main.cf
	mainCf := fmt.Sprintf(`
# Basic configuration
myhostname = %s.%s
mydomain = %s
myorigin = $mydomain
inet_interfaces = all
inet_protocols = all
mydestination = $myhostname, localhost.$mydomain, localhost, $mydomain
mynetworks = 127.0.0.0/8 [::ffff:127.0.0.0]/104 [::1]/128
home_mailbox = Maildir/

# SMTP authentication
smtpd_sasl_auth_enable = yes
smtpd_sasl_type = dovecot
smtpd_sasl_path = private/auth
smtpd_sasl_security_options = noanonymous, noplaintext
smtpd_sasl_tls_security_options = noanonymous

# TLS configuration
smtpd_tls_cert_file = /etc/ssl/certs/ssl-cert-snakeoil.pem
smtpd_tls_key_file = /etc/ssl/private/ssl-cert-snakeoil.key
smtpd_tls_security_level = may
smtp_tls_security_level = may
smtpd_tls_protocols = !SSLv2, !SSLv3

# Message size limits
message_size_limit = 25600000
mailbox_size_limit = 1000000000

# DKIM signing
milter_protocol = 2
milter_default_action = accept
smtpd_milters = inet:localhost:12301
non_smtpd_milters = inet:localhost:12301
`,
		p.Hostname,
		p.Domain,
		p.Domain,
	)
	
	// Backup original config
	_, err := client.ExecuteCommandWithOutput("cp /etc/postfix/main.cf /etc/postfix/main.cf.bak", 30*time.Second)
	if err != nil {
		return err
	}
	
	// Write new config
	cmd := fmt.Sprintf("echo '%s' > /etc/postfix/main.cf", strings.ReplaceAll(mainCf, "'", "'\\''"))
	_, err = client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Configure master.cf for submission and smtps
	masterCfExtra := `
submission inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_tls_auth_only=yes
  -o smtpd_reject_unlisted_recipient=no
  -o smtpd_client_restrictions=$mua_client_restrictions
  -o smtpd_helo_restrictions=$mua_helo_restrictions
  -o smtpd_sender_restrictions=$mua_sender_restrictions
  -o smtpd_recipient_restrictions=
  -o smtpd_relay_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING

smtps     inet  n       -       y       -       -       smtpd
  -o syslog_name=postfix/smtps
  -o smtpd_tls_wrappermode=yes
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_reject_unlisted_recipient=no
  -o smtpd_client_restrictions=$mua_client_restrictions
  -o smtpd_helo_restrictions=$mua_helo_restrictions
  -o smtpd_sender_restrictions=$mua_sender_restrictions
  -o smtpd_recipient_restrictions=
  -o smtpd_relay_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING
`
	
	cmd = fmt.Sprintf("echo '%s' >> /etc/postfix/master.cf", strings.ReplaceAll(masterCfExtra, "'", "'\\''"))
	_, err = client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	return nil
}

// configureDovecot configures Dovecot
func (p *PostfixDovecotProfile) configureDovecot(client *ssh.Client) error {
	// Configure dovecot.conf
	dovecotConf := `
# Dovecot configuration
protocols = imap pop3
listen = *
base_dir = /var/run/dovecot/
instance_name = dovecot

# SSL
ssl = yes
ssl_cert = </etc/ssl/certs/ssl-cert-snakeoil.pem
ssl_key = </etc/ssl/private/ssl-cert-snakeoil.key
ssl_protocols = !SSLv2 !SSLv3

# Logging
log_path = /var/log/dovecot.log
info_log_path = /var/log/dovecot-info.log
verbose_ssl = no

# Mail location
mail_location = maildir:~/Maildir

# Authentication
auth_mechanisms = plain login
disable_plaintext_auth = yes

# SASL
auth_socket_path = /var/run/dovecot/auth-client

!include conf.d/*.conf
`
	
	cmd := fmt.Sprintf("echo '%s' > /etc/dovecot/dovecot.conf", strings.ReplaceAll(dovecotConf, "'", "'\\''"))
	_, err := client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Configure 10-auth.conf
	authConf := `
disable_plaintext_auth = yes
auth_mechanisms = plain login
!include auth-system.conf.ext
`
	
	cmd = fmt.Sprintf("echo '%s' > /etc/dovecot/conf.d/10-auth.conf", strings.ReplaceAll(authConf, "'", "'\\''"))
	_, err = client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Configure 10-master.conf for Postfix SASL
	masterConf := `
service imap-login {
  inet_listener imap {
    port = 143
  }
  inet_listener imaps {
    port = 993
    ssl = yes
  }
}

service pop3-login {
  inet_listener pop3 {
    port = 110
  }
  inet_listener pop3s {
    port = 995
    ssl = yes
  }
}

service auth {
  unix_listener /var/run/dovecot/auth-client {
    mode = 0666
    user = postfix
    group = postfix
  }
}

service auth-worker {
  user = dovecot
}
`
	
	cmd = fmt.Sprintf("echo '%s' > /etc/dovecot/conf.d/10-master.conf", strings.ReplaceAll(masterConf, "'", "'\\''"))
	_, err = client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	return nil
}

// configureOpenDKIM configures OpenDKIM
func (p *PostfixDovecotProfile) configureOpenDKIM(client *ssh.Client) error {
	// Create directory structure
	dirs := []string{
		fmt.Sprintf("/etc/opendkim/keys/%s", p.Domain),
		"/var/run/opendkim",
	}
	
	for _, dir := range dirs {
		cmd := fmt.Sprintf("mkdir -p %s", dir)
		_, err := client.ExecuteCommandWithOutput(cmd, 30*time.Second)
		if err != nil {
			return err
		}
	}
	
	// Configure opendkim.conf
	opendkimConf := fmt.Sprintf(`
Syslog                  yes
SyslogSuccess            yes
LogWhy                  yes

# Umask
UMask                   002

# Socket
Socket                  inet:12301@localhost

# PidFile
PidFile                 /var/run/opendkim/opendkim.pid

# Mode
Mode                    sv

# Signing table
SigningTable            refile:/etc/opendkim/SigningTable

# Key table
KeyTable                refile:/etc/opendkim/KeyTable

# External ignore list
ExternalIgnoreList      refile:/etc/opendkim/ExternalIgnoreList

# Internal hosts
InternalHosts           refile:/etc/opendkim/InternalHosts

# Trusted hosts
TrustAnchorsFile        /etc/opendkim/TrustAnchors

# Key settings
KeyFile                 /etc/opendkim/keys/%s/%s.private
Selector                %s
AutoRestart             Yes
AutoRestartRate         10/1h
`,
		p.Domain,
		p.DKIMSelector,
		p.DKIMSelector,
	)
	
	cmd := fmt.Sprintf("echo '%s' > /etc/opendkim.conf", strings.ReplaceAll(opendkimConf, "'", "'\\''"))
	_, err := client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Configure SigningTable
	signingTable := fmt.Sprintf("*@%s %s._domainkey.%s\n", p.Domain, p.DKIMSelector, p.Domain)
	cmd = fmt.Sprintf("echo '%s' > /etc/opendkim/SigningTable", strings.ReplaceAll(signingTable, "'", "'\\''"))
	_, err = client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Configure KeyTable
	keyTable := fmt.Sprintf("%s._domainkey.%s %s:%s:/etc/opendkim/keys/%s/%s.private\n",
		p.DKIMSelector,
		p.Domain,
		p.Domain,
		p.DKIMSelector,
		p.Domain,
		p.DKIMSelector,
	)
	cmd = fmt.Sprintf("echo '%s' > /etc/opendkim/KeyTable", strings.ReplaceAll(keyTable, "'", "'\\''"))
	_, err = client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Configure InternalHosts
	internalHosts := fmt.Sprintf("127.0.0.1\nlocalhost\n%s\n%s.%s\n",
		p.Domain,
		p.Hostname,
		p.Domain,
	)
	cmd = fmt.Sprintf("echo '%s' > /etc/opendkim/InternalHosts", strings.ReplaceAll(internalHosts, "'", "'\\''"))
	_, err = client.ExecuteCommandWithOutput(cmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Generate DKIM key
	genKeyCmd := fmt.Sprintf("opendkim-genkey -b %d -r -s %s -d %s -D /etc/opendkim/keys/%s",
		p.DKIMKeySize,
		p.DKIMSelector,
		p.Domain,
		p.Domain,
	)
	_, err = client.ExecuteCommandWithOutput(genKeyCmd, 60*time.Second)
	if err != nil {
		return err
	}
	
	// Set permissions
	chmodCmd := fmt.Sprintf("chown -R opendkim:opendkim /etc/opendkim && chmod 600 /etc/opendkim/keys/%s/*.private", p.Domain)
	_, err = client.ExecuteCommandWithOutput(chmodCmd, 30*time.Second)
	if err != nil {
		return err
	}
	
	// Enable and start opendkim
	_, err = client.ExecuteCommandWithOutput("systemctl enable opendkim", 30*time.Second)
	if err != nil {
		return err
	}
	
	_, err = client.ExecuteCommandWithOutput("systemctl restart opendkim", 30*time.Second)
	if err != nil {
		return err
	}
	
	return nil
}