import requests from './requests';
import {GetImageResponse} from '../models';

export class ImageService {
    public get(image: string): Promise<GetImageResponse> {
        return requests.get('/images/' + "ubuntu").then(res => res.body as GetImageResponse);
    }
}
