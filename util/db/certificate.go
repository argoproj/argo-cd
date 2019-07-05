package db

import (
	"fmt"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"

	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/util/cert"
)

// A struct representing an entry in the list of SSH known hosts.
type SSHKnownHostsEntry struct {
	// Hostname the key is for
	Host string
	// The type of the key
	SubType string
	// The data of the key, including the type
	Data string
	// The SHA256 fingerprint of the key
	Fingerprint string
}

// A representation of a TLS certificate
type TLSCertificate struct {
	// Subject of the certificate
	Subject string
	// Issuer of the certificate
	Issuer string
	// Certificate data
	Data string
}

// Helper struct for certificate selection
type CertificateListSelector struct {
	// Pattern to match the hostname with
	HostNamePattern string
	// Type of certificate to match
	CertType string
	// Subtype of certificate to match
	CertSubType string
}

// Get a list of all configured repository certificates matching the given
// selector.
func (db *db) ListRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appsv1.RepositoryCertificateList, error) {

	// selector may be given as nil, but we need at least an empty data structure
	// so we create it if necessary.
	if selector == nil {
		selector = &CertificateListSelector{}
	}

	certificates := make([]appsv1.RepositoryCertificate, 0)

	if selector.CertType == "" || selector.CertType == "*" || selector.CertType == "ssh" {
		sshKnownHosts, err := db.getSSHKnownHostsData(ctx)
		if err != nil {
			return nil, err
		}

		for _, entry := range sshKnownHosts {
			if certutil.MatchHostName(entry.Host, selector.HostNamePattern) {
				certificates = append(certificates, appsv1.RepositoryCertificate{
					ServerName:      entry.Host,
					CertType:        "ssh",
					CertCipher:      entry.SubType,
					CertData:        []byte(entry.Data),
					CertFingerprint: entry.Fingerprint,
				})
			}
		}
	}

	if selector.CertType == "" || selector.CertType == "https" || selector.CertType == "tls" {
		tlsCertificates, err := db.getTLSCertificateData(ctx)
		if err != nil {
			return nil, err
		}
		for _, entry := range tlsCertificates {
			if certutil.MatchHostName(entry.Subject, selector.HostNamePattern) {
				certificates = append(certificates, appsv1.RepositoryCertificate{
					ServerName: entry.Subject,
					CertType:   "https",
					CertData:   []byte(entry.Data),
				})
			}
		}
	}

	return &appsv1.RepositoryCertificateList{
		Items: certificates,
	}, nil
}

// Get a single certificate from the datastore
func (db *db) GetRepoCertificate(ctx context.Context, serverType string, serverName string) (*appsv1.RepositoryCertificate, error) {
	if serverType == "ssh" {
		sshKnownHostsList, err := db.getSSHKnownHostsData(ctx)
		if err != nil {
			return nil, err
		}
		for _, entry := range sshKnownHostsList {
			if entry.Host == serverName {
				repo := &appsv1.RepositoryCertificate{
					ServerName:      entry.Host,
					CertType:        "ssh",
					CertCipher:      entry.SubType,
					CertData:        []byte(entry.Data),
					CertFingerprint: entry.Fingerprint,
				}
				return repo, nil
			}
		}
	}

	// Fail
	return nil, nil
}

// Create one or more repository certificates and returns a list of certificates
// actually created.
func (db *db) CreateRepoCertificate(ctx context.Context, certificates *appsv1.RepositoryCertificateList) (*appsv1.RepositoryCertificateList, error) {
	var (
		saveSSHData bool = false
		saveTLSData bool = false
	)

	sshKnownHostsList, err := db.getSSHKnownHostsData(ctx)
	if err != nil {
		return nil, err
	}

	tlsCertificates, err := db.getTLSCertificateData(ctx)
	if err != nil {
		return nil, err
	}

	// This will hold the final list of certificates that have been created
	created := make([]appsv1.RepositoryCertificate, 0)

	// Each request can contain multiple certificates of different types, so we
	// make sure to handle each request accordingly.
	for _, certificate := range certificates.Items {
		if certificate.CertType == "ssh" {
			// Check whether known hosts entry already exists
			for _, entry := range sshKnownHostsList {
				if entry.Host == certificate.ServerName && entry.SubType == certificate.CertCipher {
					return nil, fmt.Errorf("Duplicate SSH key sent for '%s' (subtype: '%s')", entry.Host, entry.SubType)
				}
			}

			// Make sure that we received a valid public host key by parsing it
			_, _, rawKeyData, _, _, err := ssh.ParseKnownHosts([]byte(fmt.Sprintf("%s %s %s", certificate.ServerName, certificate.CertCipher, certificate.CertData)))
			if err != nil {
				return nil, err
			}

			sshKnownHostsList = append(sshKnownHostsList, SSHKnownHostsEntry{
				Host:    certificate.ServerName,
				Data:    string(certificate.CertData),
				SubType: certificate.CertCipher,
			})

			certificate.CertFingerprint = certutil.SSHFingerprintSHA256(rawKeyData)
			created = append(created, certificate)

			saveSSHData = true

		} else if certificate.CertType == "https" {
			var tlsCertificate *TLSCertificate = nil
			for _, entry := range tlsCertificates {
				if entry.Subject == certificate.ServerName {
					tlsCertificate = entry
					break
					//return nil, errors.New(fmt.Sprintf("Duplicate TLS certificate sent for '%s'", entry.Subject))
				}
			}

			if tlsCertificate == nil {
				tlsCertificate = &TLSCertificate{
					Subject: certificate.ServerName,
					Data:    string(certificate.CertData),
				}
				tlsCertificates = append(tlsCertificates, tlsCertificate)
			} else {
				tlsCertificate.Data += "\n" + string(certificate.CertData)
			}

			created = append(created, certificate)
			saveTLSData = true
		} else {
			// Invalid/unknown certificate type
			return nil, fmt.Errorf("Unknown certificate type: %s", certificate.CertType)
		}
	}

	if saveSSHData {
		err = db.settingsMgr.SaveSSHKnownHostsData(ctx, knownHostsDataToStrings(sshKnownHostsList))
		if err != nil {
			return nil, err
		}
	}

	if saveTLSData {
		err = db.settingsMgr.SaveTLSCertificateData(ctx, tlsCertificatesToMap(tlsCertificates))
		if err != nil {
			return nil, err
		}
	}

	return &appsv1.RepositoryCertificateList{Items: created}, nil
}

// Batch remove configured certificates according to the selector query
func (db *db) RemoveRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appsv1.RepositoryCertificateList, error) {
	var (
		knownHostsOld      []SSHKnownHostsEntry
		knownHostsNew      []SSHKnownHostsEntry
		tlsCertificatesOld []*TLSCertificate
		tlsCertificatesNew []*TLSCertificate
		err                error
	)

	removed := &appsv1.RepositoryCertificateList{
		Items: make([]appsv1.RepositoryCertificate, 0),
	}

	if selector.CertType == "" || selector.CertType == "ssh" || selector.CertType == "*" {
		knownHostsOld, err = db.getSSHKnownHostsData(ctx)
		if err != nil {
			return nil, err
		}
		knownHostsNew = make([]SSHKnownHostsEntry, 0)

		for _, entry := range knownHostsOld {
			if matchSSHKnownHostsEntry(entry, selector) {
				removed.Items = append(removed.Items, appsv1.RepositoryCertificate{
					ServerName: entry.Host,
					CertType:   "ssh",
					CertCipher: entry.SubType,
					CertData:   []byte(entry.Data),
				})
			} else {
				knownHostsNew = append(knownHostsNew, entry)
			}
		}
	}

	if selector.CertType == "" || selector.CertType == "*" || selector.CertType == "https" || selector.CertType == "tls" {
		tlsCertificatesOld, err = db.getTLSCertificateData(ctx)
		if err != nil {
			return nil, err
		}
		tlsCertificatesNew = make([]*TLSCertificate, 0)
		for _, entry := range tlsCertificatesOld {
			if certutil.MatchHostName(entry.Subject, selector.HostNamePattern) {
				removed.Items = append(removed.Items, appsv1.RepositoryCertificate{
					ServerName: entry.Subject,
					CertType:   "https",
					CertData:   []byte(entry.Data),
				})
			} else {
				tlsCertificatesNew = append(tlsCertificatesNew, entry)
			}
		}
	}

	if len(knownHostsNew) < len(knownHostsOld) {
		err = db.settingsMgr.SaveSSHKnownHostsData(ctx, knownHostsDataToStrings(knownHostsNew))
		if err != nil {
			return nil, err
		}
	}

	if len(tlsCertificatesNew) < len(tlsCertificatesOld) {
		err = db.settingsMgr.SaveTLSCertificateData(ctx, tlsCertificatesToMap(tlsCertificatesNew))
		if err != nil {
			return nil, err
		}
	}

	return removed, nil
}

// Converts list of known hosts data to array of strings, suitable for storing
// in a known_hosts file for SSH.
func knownHostsDataToStrings(knownHostsList []SSHKnownHostsEntry) []string {
	knownHostsData := make([]string, 0)
	for _, entry := range knownHostsList {
		knownHostsData = append(knownHostsData, fmt.Sprintf("%s %s %s", entry.Host, entry.SubType, entry.Data))
	}
	return knownHostsData
}

// Converts list of TLS certificates to a map whose key will be the certificate
// subject and the data will be a string containing TLS certificate data as PEM
func tlsCertificatesToMap(tlsCertificates []*TLSCertificate) map[string]string {
	certMap := make(map[string]string)
	for _, entry := range tlsCertificates {
		certMap[entry.Subject] = entry.Data
	}
	return certMap
}

// Get the TLS certificate data from the config map
func (db *db) getTLSCertificateData(ctx context.Context) ([]*TLSCertificate, error) {
	certificates := make([]*TLSCertificate, 0)
	certCM, err := db.settingsMgr.GetNamedConfigMap(common.ArgoCDTLSCertsConfigMapName)
	if err != nil {
		return nil, err
	}
	for key, entry := range certCM.Data {
		certificates = append(certificates, &TLSCertificate{Subject: key, Data: entry})
	}

	return certificates, nil
}

// Gets the SSH known host data from ConfigMap and parse it into an array of
// SSHKnownHostEntry structs.
func (db *db) getSSHKnownHostsData(ctx context.Context) ([]SSHKnownHostsEntry, error) {
	certCM, err := db.settingsMgr.GetNamedConfigMap(common.ArgoCDKnownHostsConfigMapName)
	if err != nil {
		return nil, err
	}

	sshKnownHostsData := certCM.Data["ssh_known_hosts"]
	entries := make([]SSHKnownHostsEntry, 0)

	// ssh_known_hosts data contains one key per line, so we must iterate over
	// the whole data to get all keys.
	//
	// We validate the data found to a certain extent before we accept them as
	// entry into our list to be returned.
	//
	sshKnownHostsEntries, err := certutil.ParseSSHKnownHostsFromData(sshKnownHostsData)
	if err != nil {
		return nil, err
	}

	for _, entry := range sshKnownHostsEntries {
		hostname, subType, keyData, err := certutil.TokenizeSSHKnownHostsEntry(entry)
		if err != nil {
			return nil, err
		}
		entries = append(entries, SSHKnownHostsEntry{
			Host:    hostname,
			SubType: subType,
			Data:    fmt.Sprintf("%s %s", subType, keyData),
		})
	}

	return entries, nil
}

func matchSSHKnownHostsEntry(entry SSHKnownHostsEntry, selector *CertificateListSelector) bool {
	return certutil.MatchHostName(entry.Host, selector.HostNamePattern) && (selector.CertSubType == "" || selector.CertSubType == "*" || selector.CertSubType == entry.SubType)
}
