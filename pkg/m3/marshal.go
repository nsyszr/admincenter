package m3

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Marshal outputs an N3 ASCII configuration
func (cfg *Config) Marshal() string {
	var w bytes.Buffer

	marshalAdministration(&w, "administration", cfg.Administration)

	return w.String()
}

type marshalSection func(io.Writer, string, interface{})

func writeStringKVPair(w io.Writer, prefix, key string, value *string) {
	if value != nil {
		io.WriteString(w, fmt.Sprintf("%s.%s=%s\n", prefix, key, *value))
	}
}

func writeMultilineStringKVPair(w io.Writer, prefix, key string, value *string) {
	if value != nil {
		io.WriteString(w, fmt.Sprintf("%s.%s=%s\n", prefix, key, strings.Trim(*value, " \n\t\r")))
	}
}

func writeBoolKVPair(w io.Writer, prefix, key string, value *bool) {
	if value != nil {
		b := 0
		if *value {
			b = 1
		}
		io.WriteString(w, fmt.Sprintf("%s.%s=%d\n", prefix, key, b))
	}
}

func writeIntKVPair(w io.Writer, prefix, key string, value *int) {
	if value != nil {
		io.WriteString(w, fmt.Sprintf("%s.%s=%d\n", prefix, key, *value))
	}
}

func writeSection(w io.Writer, prefix, section string, in interface{}, fn marshalSection) {
	if in != nil {
		p := fmt.Sprintf("%s.%s", prefix, section)
		fn(w, p, in)
	}
}

func writeIndexedSection(w io.Writer, prefix, section string, index int, in interface{}, fn marshalSection) {
	if in != nil {
		p := fmt.Sprintf("%s.%s[%d]", prefix, section, index)
		fn(w, p, in)
	}
}

func marshalAdministration(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministration)
	writeSection(w, prefix, "users", section.Users, marshalAdministrationUsers)
	writeSection(w, prefix, "radius", section.Radius, marshalAdministrationRadius)
	writeSection(w, prefix, "webinterface", section.WebInterface, marshalAdministrationWebInterface)
	writeSection(w, prefix, "cli", section.CLI, marshalAdministrationCLI)
	writeSection(w, prefix, "hostnames", section.Hostnames, marshalAdministrationHostnames)
	writeSection(w, prefix, "certificates", section.Certificates, marshalAdministrationCertificates)
}

func marshalAdministrationUsers(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationUsers)
	writeBoolKVPair(w, prefix, "password_hashes", section.PasswordHashes)
	for i, user := range section.User {
		writeIndexedSection(w, prefix, "user", i+1, user, marshalAdministrationUsersUser)
	}
}

func marshalAdministrationUsersUser(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationUsersUser)
	writeBoolKVPair(w, prefix, "active", section.Active)
	writeStringKVPair(w, prefix, "username", section.Username)
	writeStringKVPair(w, prefix, "password", section.Password)
	writeStringKVPair(w, prefix, "group", section.Group)
}

func marshalAdministrationRadius(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationRadius)
	writeBoolKVPair(w, prefix, "active", section.Active)
	writeStringKVPair(w, prefix, "server", section.Server)
	writeIntKVPair(w, prefix, "port", section.Port)
	writeStringKVPair(w, prefix, "secret", section.Secret)
	writeStringKVPair(w, prefix, "default_group", section.DefaultGroup)
}

func marshalAdministrationWebInterface(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationWebInterface)
	writeBoolKVPair(w, prefix, "active", section.Active)
	writeIntKVPair(w, prefix, "http_port", section.HTTPPort)
	writeIntKVPair(w, prefix, "https_port", section.HTTPSPort)
	writeStringKVPair(w, prefix, "https_cert", section.HTTPSCertificate)
	writeStringKVPair(w, prefix, "https_key", section.HTTPSPrivateKey)
	writeIntKVPair(w, prefix, "session_timeout", section.SessionTimeout)
	writeBoolKVPair(w, prefix, "default_log_reverse", section.DefaultLogReverse)
	writeStringKVPair(w, prefix, "default_language", section.DefaultLanguage)
	writeIntKVPair(w, prefix, "default_refresh_interval", section.DefaultRefreshInterval)
	writeIntKVPair(w, prefix, "default_log_lines", section.DefaultLogLines)
	writeBoolKVPair(w, prefix, "active_https", section.ActiveHTTPS)
	writeBoolKVPair(w, prefix, "active_https", section.ActiveClientAuth)
	writeStringKVPair(w, prefix, "https_clientauth_ca", section.HTTPSClientAuthCA)
	writeStringKVPair(w, prefix, "https_clientauth_crl", section.HTTPSClientAuthCRL)
	writeStringKVPair(w, prefix, "https_clientauth_group", section.HTTPSClientAuthGroup)
}

func marshalAdministrationCLI(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationCLI)
	writeBoolKVPair(w, prefix, "telnet_active", section.TelnetActive)
	writeBoolKVPair(w, prefix, "ssh_active", section.SSHActive)
	writeIntKVPair(w, prefix, "telnet_port", section.TelnetPort)
	writeIntKVPair(w, prefix, "ssh_port", section.SSHPort)
	writeStringKVPair(w, prefix, "prompt", section.Prompt)
	writeStringKVPair(w, prefix, "key", section.Key)
}

func marshalAdministrationHostnames(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationHostnames)
	writeStringKVPair(w, prefix, "hostname", section.Hostname)
	writeStringKVPair(w, prefix, "domainname", section.Domainname)
	writeStringKVPair(w, prefix, "location", section.Location)
}

func marshalAdministrationCertificates(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationCertificates)
	writeSection(w, prefix, "ca_certs", section.CACerts, marshalAdministrationCertificatesCACerts)
	writeSection(w, prefix, "dh_params", section.DHParams, marshalAdministrationCertificatesDHParams)
}

func marshalAdministrationCertificatesCACerts(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationCertificatesCACerts)
	for i, ca := range section.CA {
		writeIndexedSection(w, prefix, "ca", i+1, ca, marshalAdministrationCertificatesCACertsCA)
	}
}

func marshalAdministrationCertificatesCACertsCA(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationCertificatesCACertsCA)
	writeStringKVPair(w, prefix, "name", section.Name)
	writeMultilineStringKVPair(w, prefix, "ca_certificate", section.CACertificate)
	writeStringKVPair(w, prefix, "description", section.Description)
	writeBoolKVPair(w, prefix, "downloadable", section.Downloadable)
}

func marshalAdministrationCertificatesDHParams(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationCertificatesDHParams)
	for i, dhParam := range section.DHParam {
		writeIndexedSection(w, prefix, "dh_param", i+1, dhParam, marshalAdministrationCertificatesDHParamsDHParam)
	}
}

func marshalAdministrationCertificatesDHParamsDHParam(w io.Writer, prefix string, in interface{}) {
	if in == nil {
		return
	}

	section := in.(*ConfigAdministrationCertificatesDHParamsDHParam)
	writeStringKVPair(w, prefix, "name", section.Name)
	writeMultilineStringKVPair(w, prefix, "dh_params", section.DHParams)
	writeStringKVPair(w, prefix, "description", section.Description)
}
