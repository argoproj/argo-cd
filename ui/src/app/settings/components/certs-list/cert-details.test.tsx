import * as React from 'react';
import {render, screen} from '@testing-library/react';

import * as models from '../../../shared/models';
import {CertDetails} from './cert-details';

const sshCert: models.RepoCert = {
    serverName: 'github.com',
    certType: 'ssh',
    certSubType: 'ssh-ed25519',
    certData: btoa('AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl'),
    certInfo: 'SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU'
};

const tlsCert: models.RepoCert = {
    serverName: 'cd.example.com',
    certType: 'https',
    certSubType: 'rsa',
    certData: btoa('-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n'),
    certInfo: 'CN=cd.example.com'
};

describe('CertDetails', () => {
    it('shows an SSH key as a complete known_hosts line', () => {
        render(<CertDetails certs={[sshCert]} />);

        expect(screen.getByText('SSH KNOWN HOSTS ENTRY')).toBeInTheDocument();
        expect(screen.getByText('FINGERPRINT:')).toBeInTheDocument();
        expect(screen.getByText(sshCert.certInfo)).toBeInTheDocument();
        expect(screen.getByText('github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl')).toBeInTheDocument();
    });

    it('shows the decoded PEM data of a TLS certificate', () => {
        render(<CertDetails certs={[tlsCert]} />);

        expect(screen.getByText('TLS CERTIFICATE')).toBeInTheDocument();
        expect(screen.getByText('SUBJECT:')).toBeInTheDocument();
        expect(screen.getByText('CN=cd.example.com')).toBeInTheDocument();
        expect(screen.getByText(/-----BEGIN CERTIFICATE-----/)).toHaveTextContent('-----BEGIN CERTIFICATE----- MIIB -----END CERTIFICATE-----');
    });

    it('shows every certificate of a bundle', () => {
        render(<CertDetails certs={[tlsCert, {...tlsCert, certInfo: 'CN=intermediate'}]} />);

        expect(screen.getAllByText('TLS CERTIFICATE')).toHaveLength(2);
        expect(screen.getByText('CN=intermediate')).toBeInTheDocument();
    });
});
