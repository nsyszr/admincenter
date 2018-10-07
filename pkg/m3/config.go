package m3

type Config struct {
	Administration *ConfigAdministration `yaml:"administration,omitempty"`
}

type ConfigAdministration struct {
	Users        *ConfigAdministrationUsers        `yaml:"users,omitempty"`
	Radius       *ConfigAdministrationRadius       `yaml:"radius,omitempty"`
	WebInterface *ConfigAdministrationWebInterface `yaml:"webinterface,omitempty"`
	CLI          *ConfigAdministrationCLI          `yaml:"cli,omitempty"`
	Hostnames    *ConfigAdministrationHostnames    `yaml:"hostnames,omitempty"`
	Certificates *ConfigAdministrationCertificates `yaml:"certificates,omitempty"`
}

type ConfigAdministrationUsers struct {
	PasswordHashes *bool                            `yaml:"password_hashes"`
	User           []*ConfigAdministrationUsersUser `yaml:"user,omitempty"`
}

type ConfigAdministrationUsersUser struct {
	Active   *bool   `yaml:"active"`
	Username *string `yaml:"username"`
	Password *string `yaml:"password"`
	Group    *string `yaml:"group"`
}

type ConfigAdministrationRadius struct {
	Active       *bool   `yaml:"active"`
	Server       *string `yaml:"server"`
	Port         *int    `yaml:"port"`
	Secret       *string `yaml:"secret"`
	DefaultGroup *string `yaml:"default_group"`
}

type ConfigAdministrationWebInterface struct {
	Active                 *bool   `yaml:"active"`
	HTTPPort               *int    `yaml:"http_port"`
	HTTPSPort              *int    `yaml:"https_port"`
	HTTPSCertificate       *string `yaml:"https_cert"`
	HTTPSPrivateKey        *string `yaml:"https_key"`
	SessionTimeout         *int    `yaml:"session_timeout"`
	DefaultLogReverse      *bool   `yaml:"default_log_reverse"`
	DefaultLanguage        *string `yaml:"default_language"`
	DefaultRefreshInterval *int    `yaml:"default_refresh_interval"`
	DefaultLogLines        *int    `yaml:"default_log_lines"`
	ActiveHTTPS            *bool   `yaml:"active_https"`
	ActiveClientAuth       *bool   `yaml:"active_clientauth"`
	HTTPSClientAuthCA      *string `yaml:"https_clientauth_ca"`
	HTTPSClientAuthCRL     *string `yaml:"https_clientauth_crl"`
	HTTPSClientAuthGroup   *string `yaml:"https_clientauth_group"`
}

type ConfigAdministrationCLI struct {
	TelnetActive *bool   `yaml:"telnet_active"`
	SSHActive    *bool   `yaml:"ssh_active"`
	TelnetPort   *int    `yaml:"telnet_port"`
	SSHPort      *int    `yaml:"ssh_port"`
	Prompt       *string `yaml:"prompt"`
	Key          *string `yaml:"key"`
}

type ConfigAdministrationHostnames struct {
	Hostname   *string `yaml:"hostname"`
	Domainname *string `yaml:"domainname"`
	Location   *string `yaml:"location"`
}

type ConfigAdministrationCertificates struct {
	CACerts  *ConfigAdministrationCertificatesCACerts  `yaml:"ca_certs,omitempty"`
	DHParams *ConfigAdministrationCertificatesDHParams `yaml:"dh_params,omitempty"`
}

type ConfigAdministrationCertificatesCACerts struct {
	CA []*ConfigAdministrationCertificatesCACertsCA `yaml:"ca,omitempty"`
}

type ConfigAdministrationCertificatesCACertsCA struct {
	Name          *string `yaml:"name"`
	CACertificate *string `yaml:"ca_certificate"`
	Description   *string `yaml:"description"`
	Downloadable  *bool   `yaml:"downloadable"`
}

type ConfigAdministrationCertificatesDHParams struct {
	DHParam []*ConfigAdministrationCertificatesDHParamsDHParam `yaml:"dh_param,omitempty"`
}

type ConfigAdministrationCertificatesDHParamsDHParam struct {
	Name        *string `yaml:"name"`
	DHParams    *string `yaml:"dh_params"`
	Description *string `yaml:"description"`
}
