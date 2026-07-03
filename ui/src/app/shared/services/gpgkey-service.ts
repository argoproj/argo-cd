import * as models from '../models';
import requests from './requests';

export class GnuPGPublicKeyService {
    public list(): Promise<models.GnuPGPublicKey[]> {
        return requests
            .get('/gpgkeys')
            .then(res => res.body as models.GnuPGPublicKeyList)
            .then(list => list.items || []);
    }

    public create(publickey: models.GnuPGPublicKey): Promise<models.GnuPGPublicKeyList> {
        return requests
            .post('/gpgkeys')
            .send(publickey)
            .then(res => res.body as models.GnuPGPublicKeyList);
    }

    public delete(keyID: string): Promise<models.GnuPGPublicKey> {
        return requests
            .delete('/gpgkeys')
            .query({keyID})
            .send()
            .then(res => res.body as models.GnuPGPublicKey);
    }
}
