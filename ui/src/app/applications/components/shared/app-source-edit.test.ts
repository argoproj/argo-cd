import * as models from '../../../shared/models';
import {isRefOnlySource} from './app-source-edit';

describe('isRefOnlySource', () => {
    test.each([
        ['Git source with ref only', {repoURL: 'https://git.example.com/values.git', ref: 'values'}, true],
        ['Git source with ref and path', {repoURL: 'https://git.example.com/values.git', ref: 'values', path: 'manifests'}, false],
        ['Helm source with ref', {repoURL: 'https://charts.example.com', ref: 'values', chart: 'application'}, false],
        ['OCI source with ref', {repoURL: 'oci://registry.example.com/application', ref: 'values'}, false]
    ])('%s', (_name, source, expected) => {
        expect(isRefOnlySource(source as models.ApplicationSource)).toBe(expected);
    });
});
