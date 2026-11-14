import {resources} from './resources';

export const ResourceLabel = ({kind}: {kind: string}) => {
    return resources.get(kind) || kind.toLowerCase();
};
