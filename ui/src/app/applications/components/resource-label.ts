import {resources} from './resources';

const trunc = (text: string, n: number) => (text.length <= n ? text : text.substring(0, n));

export const ResourceLabel = ({kind}: {kind: string}) => {
    const label = resources.get(kind);
    if (label !== undefined) {
        return label;
    }

    // so, it is a crd - just use it's initials
    const initials = kind.match(/[A-Z]/g).map(i => i.toLowerCase());
    const words = kind.match(/[A-Z][a-z]+/g).map(i => i.toLowerCase());
    switch (initials.length) {
        case 1:
            return trunc(words[0], 7);
        case 2:
            return initials[0] + '-' + trunc(words[1], 5);
        default:
            return trunc(initials.join(''), 7);
    }
};
