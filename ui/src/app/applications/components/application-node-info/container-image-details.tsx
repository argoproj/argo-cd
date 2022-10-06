import * as React from 'react';
import {useEffect, useState} from 'react';
import {services} from '../../../shared/services';
import {GetImageResponse} from '../../../shared/models';
import {ErrorNotification} from 'argo-ui';
import Moment from 'react-moment';

export const ContainerImageDetails = ({image}: {image: string}) => {
    const [error, setError] = useState<Error>();
    const [resp, setResp] = useState<GetImageResponse>();
    useEffect(() => {
        services.images
            .get(image)
            .then(setResp)
            .catch(setError);
    }, [image]);

    const parts = image.replace(/:.*/, '').split('/');
    const repo = (parts.length === 1 ? '_/' : '') + parts.join('/');
    // TODO: won't work for private repos, but you get the idea
    const path = 'https://hub.docker.com/' + repo;

    if (error) return <ErrorNotification title={'Failed to get image ' + image} e={error} />;

    if (resp)
        return (
            <>
                <div>
                    Created: <Moment fromNow={true}>{resp.image?.created}</Moment>
                </div>
                <div>Author: {resp.image?.author}</div>
                <div>
                    Command: <code>{resp.image.config.command?.join(' ') || '-'}</code>
                </div>
                <div>
                    Entrypoint: <code>{resp.image.config.entrypoint?.join(' ') || '-'}</code>
                </div>
                <div>
                    Labels:{' '}
                    <code>
                        {Object.entries(resp.image.config.labels || {})
                            .map(([k, v]) => k + '=' + v)
                            .join(',')}
                    </code>
                </div>
                <div>
                    <a href={path}>See more in your container registry</a>
                </div>
            </>
        );

    return <>Loading...</>;
};
