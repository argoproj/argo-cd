import * as React from 'react';

import * as models from '../../../shared/models';

require('./cert-details.scss');

// certData is a byte array in the API, so it is transferred base64 encoded.
const decodeCertData = (certData: string): string => {
    try {
        return atob(certData || '');
    } catch {
        return certData;
    }
};

const certTitle = (cert: models.RepoCert) => (cert.certType === 'ssh' ? 'SSH KNOWN HOSTS ENTRY' : 'TLS CERTIFICATE');

// The list result carries the SSH fingerprint or the X.509 subject in certInfo.
const certInfoLabel = (cert: models.RepoCert) => (cert.certType === 'ssh' ? 'FINGERPRINT:' : 'SUBJECT:');

const certDataText = (cert: models.RepoCert) => {
    const data = decodeCertData(cert.certData).trim();
    // Show SSH keys as a complete known_hosts line, so it can be copied as is.
    return cert.certType === 'ssh' ? `${cert.serverName} ${cert.certSubType} ${data}` : data;
};

export const CertDetails = ({certs}: {certs: models.RepoCert[]}) => (
    <div className='cert-details'>
        {certs.map((cert, i) => (
            <div className='white-box' key={`${cert.certType}_${cert.certSubType}_${i}`}>
                <p>{certTitle(cert)}</p>
                <div className='white-box__details'>
                    <div className='row white-box__details-row'>
                        <div className='columns small-3'>SERVER NAME:</div>
                        <div className='columns small-9'>{cert.serverName}</div>
                    </div>
                    <div className='row white-box__details-row'>
                        <div className='columns small-3'>TYPE:</div>
                        <div className='columns small-9'>
                            {cert.certType} {cert.certSubType}
                        </div>
                    </div>
                    <div className='row white-box__details-row'>
                        <div className='columns small-3'>{certInfoLabel(cert)}</div>
                        <div className='columns small-9'>{cert.certInfo}</div>
                    </div>
                    <div className='row white-box__details-row'>
                        <div className='columns small-3'>DATA:</div>
                        <div className='columns small-9'>
                            <pre className='cert-details__data'>{certDataText(cert)}</pre>
                        </div>
                    </div>
                </div>
            </div>
        ))}
    </div>
);
