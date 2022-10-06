import * as React from 'react';
import {Tooltip} from 'argo-ui/v2';
import {ContainerImageDetails} from './container-image-details';

export const ContainerImage = ({image, className}: {image: string; className: string}) => {
    return (
        <span key={image}>
            <Tooltip content={<ContainerImageDetails image={image} />}>
                <span className={className}>{image}</span>
            </Tooltip>
        </span>
    );
};
