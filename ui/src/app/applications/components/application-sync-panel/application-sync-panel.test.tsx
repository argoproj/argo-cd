import * as models from '../../../shared/models';

// resources-pre-selection imports nodeKey from applications/components/utils, which
// pulls in ESM-only deps (lodash-es) that jest does not transform, so mock it with
// a plain nodeKey.
jest.mock('../utils', () => ({
    nodeKey: (n: {group: string; kind: string; namespace: string; name: string}) => [n.group, n.kind, n.namespace, n.name].join('/')
}));

import {resourcesPreSelection} from './resources-pre-selection';

const res = (group: string, kind: string, namespace: string, name: string): models.ResourceStatus => ({group, kind, namespace, name} as models.ResourceStatus);
const key = (r: models.ResourceStatus): string => [r.group, r.kind, r.namespace, r.name].join('/');

describe('resourcesPreSelection', () => {
    const a = res('apps', 'Deployment', 'default', 'a');
    const b = res('', 'Service', 'default', 'b');
    const c = res('apps', 'Deployment', 'default', 'c');
    const resources = [a, b, c];

    it("selects everything for 'all' (top-bar Sync)", () => {
        expect(resourcesPreSelection(resources, 'all')).toEqual([true, true, true]);
    });

    it('selects only the matching single resource (per-resource Sync menu)', () => {
        expect(resourcesPreSelection(resources, key(b))).toEqual([false, true, false]);
    });

    it('selects exactly the comma-separated subset (Sync selected from the diff)', () => {
        expect(resourcesPreSelection(resources, `${key(a)},${key(c)}`)).toEqual([true, false, true]);
    });

    it('keeps the historic fallback: a value matching nothing selects everything', () => {
        expect(resourcesPreSelection(resources, 'does/not/match/anything')).toEqual([true, true, true]);
    });

    it('selects everything when the value is empty', () => {
        expect(resourcesPreSelection(resources, '')).toEqual([true, true, true]);
    });
});
