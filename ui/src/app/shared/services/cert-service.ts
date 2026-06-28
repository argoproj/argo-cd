import * as models from '../models';
import requests from './requests';

export class CertificatesService {
    public list(): Promise<models.RepoCert[]> {
        return requests
            .get('/certificates')
            .then(res => res.body as models.RepoCertList)
            .then(list => list.items || []);
    }

    public create(certificates: models.RepoCertList): Promise<models.RepoCertList> {
        return requests
            .post('/certificates')
            .send(certificates)
            .then(res => res.body as models.RepoCertList);
    }

    public delete(serverName: string, certType: string, certSubType: string): Promise<models.RepoCert> {
        return requests
            .delete('/certificates')
            .query({hostNamePattern: serverName, certType, certSubType})
            .send()
            .then(res => res.body as models.RepoCert);
    }
}
