import * as React from 'react';
import {render, screen} from '@testing-library/react';

import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {CertDetails, CertDetailsPanel, certDetailsFromQuery} from './cert-details';

jest.mock('../../../shared/services', () => ({
    services: {certs: {get: jest.fn()}}
}));

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

describe('certDetailsFromQuery', () => {
    it('reads the selected server and type from a shared link', () => {
        const query = new URLSearchParams('certDetails=%5Bssh.github.com%5D%3A443&certDetailsType=ssh');
        expect(certDetailsFromQuery(query)).toEqual({serverName: '[ssh.github.com]:443', certType: 'ssh'});
    });

    it('selects nothing when the link has no details', () => {
        expect(certDetailsFromQuery(new URLSearchParams('certType=https'))).toEqual({serverName: null, certType: null});
    });
});

describe('CertDetailsPanel', () => {
    beforeEach(() => jest.clearAllMocks());

    it('loads and shows the certificates of the selected server', async () => {
        (services.certs.get as jest.Mock).mockResolvedValue([sshCert]);

        render(<CertDetailsPanel selection={{serverName: 'github.com', certType: 'ssh'}} />);

        expect(await screen.findByText('SSH KNOWN HOSTS ENTRY')).toBeInTheDocument();
        expect(services.certs.get).toHaveBeenCalledWith('github.com', 'ssh');
    });

    it('requests all types when none is selected', async () => {
        (services.certs.get as jest.Mock).mockResolvedValue([tlsCert]);

        render(<CertDetailsPanel selection={{serverName: 'cd.example.com', certType: null}} />);

        expect(await screen.findByText('TLS CERTIFICATE')).toBeInTheDocument();
        expect(services.certs.get).toHaveBeenCalledWith('cd.example.com', undefined);
    });
});
