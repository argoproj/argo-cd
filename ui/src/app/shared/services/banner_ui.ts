import {BannerMessage} from '../models';
import requests from './requests';

var apiUrl = 'https://api.mocki.io/v1/725362fa';
export class BannerService {
    public banner(): Promise<BannerMessage> {
        return requests.agent.get(apiUrl).then(res => res.body as BannerMessage);
    }
}
