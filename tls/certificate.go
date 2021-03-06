package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/containous/traefik/log"
	"github.com/containous/traefik/tls/generate"
)

var (
	// MinVersion Map of allowed TLS minimum versions
	MinVersion = map[string]uint16{
		`VersionTLS10`: tls.VersionTLS10,
		`VersionTLS11`: tls.VersionTLS11,
		`VersionTLS12`: tls.VersionTLS12,
	}

	// CipherSuites Map of TLS CipherSuites from crypto/tls
	// Available CipherSuites defined at https://golang.org/pkg/crypto/tls/#pkg-constants
	CipherSuites = map[string]uint16{
		`TLS_RSA_WITH_RC4_128_SHA`:                tls.TLS_RSA_WITH_RC4_128_SHA,
		`TLS_RSA_WITH_3DES_EDE_CBC_SHA`:           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		`TLS_RSA_WITH_AES_128_CBC_SHA`:            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		`TLS_RSA_WITH_AES_256_CBC_SHA`:            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		`TLS_RSA_WITH_AES_128_CBC_SHA256`:         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		`TLS_RSA_WITH_AES_128_GCM_SHA256`:         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		`TLS_RSA_WITH_AES_256_GCM_SHA384`:         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		`TLS_ECDHE_ECDSA_WITH_RC4_128_SHA`:        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
		`TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA`:    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		`TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`:    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		`TLS_ECDHE_RSA_WITH_RC4_128_SHA`:          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		`TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA`:     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
		`TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA`:      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		`TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`:      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		`TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256`: tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		`TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256`:   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		`TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`:   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		`TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`: tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		`TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`:   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		`TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`: tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		`TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305`:    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		`TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305`:  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}
)

// Certificate holds a SSL cert/key pair
// Certs and Key could be either a file path, or the file content itself
type Certificate struct {
	CertFile FileOrContent
	KeyFile  FileOrContent
}

// Certificates defines traefik certificates type
// Certs and Keys could be either a file path, or the file content itself
type Certificates []Certificate

// FileOrContent hold a file path or content
type FileOrContent string

func (f FileOrContent) String() string {
	return string(f)
}

func (f FileOrContent) Read() ([]byte, error) {
	var content []byte
	if _, err := os.Stat(f.String()); err == nil {
		content, err = ioutil.ReadFile(f.String())
		if err != nil {
			return nil, err
		}
	} else {
		content = []byte(f)
	}
	return content, nil
}

// CreateTLSConfig creates a TLS config from Certificate structures
func (c *Certificates) CreateTLSConfig(entryPointName string) (*tls.Config, map[string]*DomainsCertificates, error) {
	config := &tls.Config{}
	domainsCertificates := make(map[string]*DomainsCertificates)
	if c.isEmpty() {
		config.Certificates = make([]tls.Certificate, 0)
		cert, err := generate.DefaultCertificate()
		if err != nil {
			return nil, nil, err
		}
		config.Certificates = append(config.Certificates, *cert)
	} else {
		for _, certificate := range *c {
			err := certificate.AppendCertificates(domainsCertificates, entryPointName)
			if err != nil {
				return nil, nil, err
			}
			for _, certDom := range domainsCertificates {
				for _, cert := range certDom.Get().(map[string]*tls.Certificate) {
					config.Certificates = append(config.Certificates, *cert)
				}
			}
		}
	}
	return config, domainsCertificates, nil
}

// isEmpty checks if the certificates list is empty
func (c *Certificates) isEmpty() bool {
	if len(*c) == 0 {
		return true
	}
	var key int
	for _, cert := range *c {
		if len(cert.CertFile.String()) != 0 && len(cert.KeyFile.String()) != 0 {
			break
		}
		key++
	}
	return key == len(*c)
}

// AppendCertificates appends a Certificate to a certificates map sorted by entrypoints
func (c *Certificate) AppendCertificates(certs map[string]*DomainsCertificates, ep string) error {

	certContent, err := c.CertFile.Read()
	if err != nil {
		return err
	}

	keyContent, err := c.KeyFile.Read()
	if err != nil {
		return err
	}
	tlsCert, err := tls.X509KeyPair(certContent, keyContent)
	if err != nil {
		return err
	}

	parsedCert, _ := x509.ParseCertificate(tlsCert.Certificate[0])

	certKey := parsedCert.Subject.CommonName
	if parsedCert.DNSNames != nil {
		sort.Strings(parsedCert.DNSNames)
		certKey += fmt.Sprintf("%s,%s", parsedCert.Subject.CommonName, strings.Join(parsedCert.DNSNames, ","))
	}

	certExists := false
	if certs[ep] == nil {
		certs[ep] = new(DomainsCertificates)
		*certs[ep] = make(map[string]*tls.Certificate)
	} else {
		for domains := range *certs[ep] {
			if domains == certKey {
				certExists = true
				break
			}
		}
	}
	if certExists {
		log.Warnf("Into EntryPoint %s, try to add certificate for domains which already have a certificate (%s). The new certificate will not be append to the EntryPoint.", ep, certKey)
	} else {
		log.Debugf("Add certificate for domains %s", certKey)
		err = certs[ep].add(certKey, &tlsCert)
	}

	return err
}

// String is the method to format the flag's value, part of the flag.Value interface.
// The String method's output will be used in diagnostics.
func (c *Certificates) String() string {
	if len(*c) == 0 {
		return ""
	}
	var result []string
	for _, certificate := range *c {
		result = append(result, certificate.CertFile.String()+","+certificate.KeyFile.String())
	}
	return strings.Join(result, ";")
}

// Set is the method to set the flag value, part of the flag.Value interface.
// Set's argument is a string to be parsed to set the flag.
// It's a comma-separated list, so we split it.
func (c *Certificates) Set(value string) error {
	certificates := strings.Split(value, ";")
	for _, certificate := range certificates {
		files := strings.Split(certificate, ",")
		if len(files) != 2 {
			return fmt.Errorf("bad certificates format: %s", value)
		}
		*c = append(*c, Certificate{
			CertFile: FileOrContent(files[0]),
			KeyFile:  FileOrContent(files[1]),
		})
	}
	return nil
}

// Type is type of the struct
func (c *Certificates) Type() string {
	return "certificates"
}
