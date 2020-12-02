import {BannerMessage} from '../models';
import requests from './requests';

const apiUrl = 'https://api.mocki.io/v1/b837272d';
export class BannerService {
    public banner(): Promise<BannerMessage> {
        return requests.agent.get(apiUrl).then(res => res.body as BannerMessage);
    }
}
