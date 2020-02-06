import * as models from '../models';
import requests from './requests';

export class CertificatesService {
    public list(): Promise<models.RepoCert[]> {
        return requests
            .get('/certificates')
            .then(res => res.body as models.RepoCertList)
            .then(list => list.items || []);
    }

    public async create(certificates: models.RepoCertList): Promise<models.RepoCertList> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .post('/certificates')
            .set(requests.csrfHeaderName, csrfToken)
            .send(certificates)
            .then(res => res.body as models.RepoCertList);
    }

    public async delete(serverName: string, certType: string, certSubType: string): Promise<models.RepoCert> {
        const csrfToken = await requests.getCsrfToken();
        return requests
            .delete('/certificates')
            .set(requests.csrfHeaderName, csrfToken)
            .query({hostNamePattern: serverName, certType, certSubType})
            .send()
            .then(res => res.body as models.RepoCert);
    }
}
