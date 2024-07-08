package db

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/v2/util/cert"
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
// selector. The list of certificates explicitly excludes the CertData of
// the certificates, and only returns the metadata including CertInfo field.
//
// The CertInfo field in the returned entries will contain the following data:
//   - For SSH keys, the SHA256 fingerprint of the key as string, prepended by
//     the string "SHA256:"
//   - For TLS certs, the Subject of the X509 cert as a string in DN notation
func (db *db) ListRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appsv1.RepositoryCertificateList, error) {
	// selector may be given as nil, but we need at least an empty data structure
	// so we create it if necessary.
	if selector == nil {
		selector = &CertificateListSelector{}
	}

	certificates := make([]appsv1.RepositoryCertificate, 0)

	// Get all SSH known host entries
	if selector.CertType == "" || selector.CertType == "*" || selector.CertType == "ssh" {
		sshKnownHosts, err := db.getSSHKnownHostsData()
		if err != nil {
			return nil, err
		}

		for _, entry := range sshKnownHosts {
			if certutil.MatchHostName(entry.Host, selector.HostNamePattern) && (selector.CertSubType == "" || selector.CertSubType == "*" || selector.CertSubType == entry.SubType) {
				certificates = append(certificates, appsv1.RepositoryCertificate{
					ServerName:  entry.Host,
					CertType:    "ssh",
					CertSubType: entry.SubType,
					CertInfo:    "SHA256:" + certutil.SSHFingerprintSHA256FromString(fmt.Sprintf("%s %s", entry.Host, entry.Data)),
				})
			}
		}
	}

	// Get all TLS certificates
	if selector.CertType == "" || selector.CertType == "*" || selector.CertType == "https" || selector.CertType == "tls" {
		tlsCertificates, err := db.getTLSCertificateData()
		if err != nil {
			return nil, err
		}
		for _, entry := range tlsCertificates {
			if certutil.MatchHostName(entry.Subject, selector.HostNamePattern) {
				pemEntries, err := certutil.ParseTLSCertificatesFromData(entry.Data)
				if err != nil {
					continue
				}
				for _, pemEntry := range pemEntries {
					var certInfo, certSubType string
					x509Data, err := certutil.DecodePEMCertificateToX509(pemEntry)
					if err != nil {
						certInfo = err.Error()
						certSubType = "invalid"
					} else {
						certInfo = x509Data.Subject.String()
						certSubType = x509Data.PublicKeyAlgorithm.String()
					}
					certificates = append(certificates, appsv1.RepositoryCertificate{
						ServerName:  entry.Subject,
						CertType:    "https",
						CertSubType: strings.ToLower(certSubType),
						CertInfo:    certInfo,
					})
				}
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
		sshKnownHostsList, err := db.getSSHKnownHostsData()
		if err != nil {
			return nil, err
		}
		for _, entry := range sshKnownHostsList {
			if entry.Host == serverName {
				repo := &appsv1.RepositoryCertificate{
					ServerName:  entry.Host,
					CertType:    "ssh",
					CertSubType: entry.SubType,
					CertData:    []byte(entry.Data),
					CertInfo:    entry.Fingerprint,
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
func (db *db) CreateRepoCertificate(ctx context.Context, certificates *appsv1.RepositoryCertificateList, upsert bool) (*appsv1.RepositoryCertificateList, error) {
	var (
		saveSSHData bool = false
		saveTLSData bool = false
	)

	sshKnownHostsList, err := db.getSSHKnownHostsData()
	if err != nil {
		return nil, err
	}

	tlsCertificates, err := db.getTLSCertificateData()
	if err != nil {
		return nil, err
	}

	// This will hold the final list of certificates that have been created
	created := make([]appsv1.RepositoryCertificate, 0)

	// Each request can contain multiple certificates of different types, so we
	// make sure to handle each request accordingly.
	for _, certificate := range certificates.Items {
		// Ensure valid repo server name was given only for https certificates.
		// For SSH known host entries, we let Go's ssh library do the validation
		// later on.
		if certificate.CertType == "https" && !certutil.IsValidHostname(certificate.ServerName, false) {
			return nil, fmt.Errorf("Invalid hostname in request: %s", certificate.ServerName)
		} else if certificate.CertType == "ssh" {
			// Matches "[hostname]:port" format
			reExtract := regexp.MustCompile(`^\[(.*)\]\:[0-9]+$`)
			matches := reExtract.FindStringSubmatch(certificate.ServerName)
			var hostnameToCheck string
			if len(matches) == 0 {
				hostnameToCheck = certificate.ServerName
			} else {
				hostnameToCheck = matches[1]
			}
			if !certutil.IsValidHostname(hostnameToCheck, false) {
				return nil, fmt.Errorf("Invalid hostname in request: %s", hostnameToCheck)
			}
		}

		if certificate.CertType == "ssh" {
			// Whether we have a new certificate entry
			newEntry := true
			// Whether we have upserted an existing certificate entry
			upserted := false

			// Check whether known hosts entry already exists. Must match hostname
			// and the key sub type (e.g. ssh-rsa). It is considered an error if we
			// already have a corresponding key and upsert was not specified.
			for _, entry := range sshKnownHostsList {
				if entry.Host == certificate.ServerName && entry.SubType == certificate.CertSubType {
					if !upsert && entry.Data != string(certificate.CertData) {
						return nil, fmt.Errorf("Key for '%s' (subtype: '%s') already exist and upsert was not specified.", entry.Host, entry.SubType)
					} else {
						// Do not add an entry on upsert, but remember if we actual did an
						// upsert.
						newEntry = false
						if entry.Data != string(certificate.CertData) {
							entry.Data = string(certificate.CertData)
							upserted = true
						}
						break
					}
				}
			}

			// Make sure that we received a valid public host key by parsing it
			_, hostnames, rawKeyData, _, _, err := ssh.ParseKnownHosts([]byte(fmt.Sprintf("%s %s %s", certificate.ServerName, certificate.CertSubType, certificate.CertData)))
			if err != nil {
				return nil, err
			}

			if len(hostnames) == 0 {
				log.Errorf("Could not parse hostname for key from token %s", certificate.ServerName)
			}

			if newEntry {
				sshKnownHostsList = append(sshKnownHostsList, &SSHKnownHostsEntry{
					Host:    hostnames[0],
					Data:    string(certificate.CertData),
					SubType: certificate.CertSubType,
				})
			}

			// If we created a new entry, or if we upserted an existing one, we need
			// to save the data and notify the consumer about the operation.
			if newEntry || upserted {
				certificate.CertInfo = certutil.SSHFingerprintSHA256(rawKeyData)
				created = append(created, certificate)
				saveSSHData = true
			}
		} else if certificate.CertType == "https" {
			var tlsCertificate *TLSCertificate = nil
			newEntry := true
			upserted := false
			pemCreated := make([]string, 0)

			for _, entry := range tlsCertificates {
				// We have an entry for this server already. Check for upsert.
				if entry.Subject == certificate.ServerName {
					newEntry = false
					if entry.Data != string(certificate.CertData) {
						if !upsert {
							return nil, fmt.Errorf("TLS certificate for server '%s' already exist and upsert was not specified.", entry.Subject)
						}
					}
					// Store pointer to this entry for later use.
					tlsCertificate = entry
					break
				}
			}

			// Check for validity of data received
			pemData, err := certutil.ParseTLSCertificatesFromData(string(certificate.CertData))
			if err != nil {
				return nil, err
			}

			// We should have at least one valid PEM entry
			if len(pemData) == 0 {
				return nil, fmt.Errorf("No valid PEM data received.")
			}

			// Make sure we have valid X509 certificates in the data
			for _, entry := range pemData {
				_, err := certutil.DecodePEMCertificateToX509(entry)
				if err != nil {
					return nil, err
				}
				pemCreated = append(pemCreated, entry)
			}

			// New certificate if pointer to existing cert is nil
			if tlsCertificate == nil {
				tlsCertificate = &TLSCertificate{
					Subject: certificate.ServerName,
					Data:    string(certificate.CertData),
				}
				tlsCertificates = append(tlsCertificates, tlsCertificate)
			} else if tlsCertificate.Data != string(certificate.CertData) {
				// We have made sure the upsert flag was set above. Now just figure out
				// again if we have to actually update the data in the existing cert.
				tlsCertificate.Data = string(certificate.CertData)
				upserted = true
			}

			if newEntry || upserted {
				// We append the certificate for every PEM entry in the request, so the
				// caller knows that we processed each single item.
				for _, entry := range pemCreated {
					created = append(created, appsv1.RepositoryCertificate{
						ServerName: certificate.ServerName,
						CertType:   "https",
						CertData:   []byte(entry),
					})
				}
				saveTLSData = true
			}
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
		knownHostsOld      []*SSHKnownHostsEntry
		knownHostsNew      []*SSHKnownHostsEntry
		tlsCertificatesOld []*TLSCertificate
		tlsCertificatesNew []*TLSCertificate
		err                error
	)

	removed := &appsv1.RepositoryCertificateList{
		Items: make([]appsv1.RepositoryCertificate, 0),
	}

	if selector.CertType == "" || selector.CertType == "ssh" || selector.CertType == "*" {
		knownHostsOld, err = db.getSSHKnownHostsData()
		if err != nil {
			return nil, err
		}
		knownHostsNew = make([]*SSHKnownHostsEntry, 0)

		for _, entry := range knownHostsOld {
			if matchSSHKnownHostsEntry(entry, selector) {
				removed.Items = append(removed.Items, appsv1.RepositoryCertificate{
					ServerName:  entry.Host,
					CertType:    "ssh",
					CertSubType: entry.SubType,
					CertData:    []byte(entry.Data),
				})
			} else {
				knownHostsNew = append(knownHostsNew, entry)
			}
		}
	}

	if selector.CertType == "" || selector.CertType == "*" || selector.CertType == "https" || selector.CertType == "tls" {
		tlsCertificatesOld, err = db.getTLSCertificateData()
		if err != nil {
			return nil, err
		}
		tlsCertificatesNew = make([]*TLSCertificate, 0)
		for _, entry := range tlsCertificatesOld {
			if certutil.MatchHostName(entry.Subject, selector.HostNamePattern) {
				// Wrap each PEM certificate into its own RepositoryCertificate object
				// so the caller knows what has been removed actually.
				//
				// The downside of this is, only valid data can be removed from the CM,
				// so if the data somehow got corrupted, it can only be removed by
				// means of editing the CM directly using e.g. kubectl.
				pemCertificates, err := certutil.ParseTLSCertificatesFromData(entry.Data)
				if err != nil {
					return nil, err
				}
				if len(pemCertificates) > 0 {
					for _, pem := range pemCertificates {
						removed.Items = append(removed.Items, appsv1.RepositoryCertificate{
							ServerName: entry.Subject,
							CertType:   "https",
							CertData:   []byte(pem),
						})
					}
				}
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
func knownHostsDataToStrings(knownHostsList []*SSHKnownHostsEntry) []string {
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
func (db *db) getTLSCertificateData() ([]*TLSCertificate, error) {
	certificates := make([]*TLSCertificate, 0)
	certCM, err := db.settingsMgr.GetConfigMapByName(common.ArgoCDTLSCertsConfigMapName)
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
func (db *db) getSSHKnownHostsData() ([]*SSHKnownHostsEntry, error) {
	certCM, err := db.settingsMgr.GetConfigMapByName(common.ArgoCDKnownHostsConfigMapName)
	if err != nil {
		return nil, err
	}

	sshKnownHostsData := certCM.Data["ssh_known_hosts"]
	entries := make([]*SSHKnownHostsEntry, 0)

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
		entries = append(entries, &SSHKnownHostsEntry{
			Host:    hostname,
			SubType: subType,
			Data:    string(keyData),
		})
	}

	return entries, nil
}

func matchSSHKnownHostsEntry(entry *SSHKnownHostsEntry, selector *CertificateListSelector) bool {
	return certutil.MatchHostName(entry.Host, selector.HostNamePattern) && (selector.CertSubType == "" || selector.CertSubType == "*" || selector.CertSubType == entry.SubType)
}
