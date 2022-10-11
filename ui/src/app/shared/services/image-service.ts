import requests from './requests';
import {GetImageResponse} from '../models';

export class ImageService {
    public get(image: string): Promise<GetImageResponse> {
        // TODO remove "ubuntu" hack
        return requests.get('/images/' + 'gcr.io/google_containers/pause').then(res => res.body as GetImageResponse);
    }
}
