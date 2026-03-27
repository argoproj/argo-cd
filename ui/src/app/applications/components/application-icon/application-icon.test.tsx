import * as React from 'react';
import * as renderer from 'react-test-renderer';
import * as models from '../../../shared/models';
import {ApplicationIcon, isValidIconUrl} from './application-icon';

jest.mock('./application-icon.scss', () => ({}));

const makeApp = (annotations?: Record<string, string>, overrides?: {chart?: string; repoURL?: string}): models.Application =>
    ({
        metadata: {name: 'test-app', annotations: annotations || {}},
        spec: {
            source: {
                repoURL: overrides?.repoURL || 'https://github.com/test/repo',
                path: '.',
                targetRevision: 'HEAD',
                chart: overrides?.chart
            },
            destination: {server: 'https://kubernetes.default.svc', namespace: 'default'}
        },
        status: {} as any
    } as models.Application);

describe('isValidIconUrl', () => {
    it('should accept valid https:// URL', () => {
        expect(isValidIconUrl('https://example.com/icon.png')).toBe(true);
    });

    it('should reject http:// URL', () => {
        expect(isValidIconUrl('http://example.com/icon.png')).toBe(false);
    });

    it('should reject data: URI', () => {
        expect(isValidIconUrl('data:image/png;base64,abc')).toBe(false);
    });
});

describe('ApplicationIcon', () => {
    it('should render <img> when annotation has valid https:// icon URL', () => {
        const app = makeApp({'argocd.argoproj.io/icon': 'https://example.com/icon.png'});
        const testRenderer = renderer.create(<ApplicationIcon app={app} />);
        const testInstance = testRenderer.root;
        const imgs = testInstance.findAllByType('img');
        expect(imgs.length).toBe(1);
        expect(imgs[0].props.src).toBe('https://example.com/icon.png');
        expect(imgs[0].props.className).toContain('application-icon--medium');
    });

    it('should render <i> fallback with argo-icon-git when no annotation is set', () => {
        const app = makeApp();
        const testRenderer = renderer.create(<ApplicationIcon app={app} />);
        const testInstance = testRenderer.root;
        const imgs = testInstance.findAllByType('img');
        expect(imgs.length).toBe(0);
        const icons = testInstance.findAllByType('i');
        expect(icons.length).toBe(1);
        expect(icons[0].props.className).toBe('icon argo-icon-git');
    });

    it('should render <i> fallback when annotation has http:// URL (rejected)', () => {
        const app = makeApp({'argocd.argoproj.io/icon': 'http://example.com/icon.png'});
        const testRenderer = renderer.create(<ApplicationIcon app={app} />);
        const testInstance = testRenderer.root;
        const imgs = testInstance.findAllByType('img');
        expect(imgs.length).toBe(0);
        const icons = testInstance.findAllByType('i');
        expect(icons.length).toBe(1);
        expect(icons[0].props.className).toBe('icon argo-icon-git');
    });

    it('should render <i> fallback with argo-icon-helm when chart is set', () => {
        const app = makeApp({}, {chart: 'my-chart'});
        const testRenderer = renderer.create(<ApplicationIcon app={app} />);
        const testInstance = testRenderer.root;
        const imgs = testInstance.findAllByType('img');
        expect(imgs.length).toBe(0);
        const icons = testInstance.findAllByType('i');
        expect(icons.length).toBe(1);
        expect(icons[0].props.className).toBe('icon argo-icon-helm');
    });

    it('should render <i> fallback with argo-icon-oci when repoURL starts with oci://', () => {
        const app = makeApp({}, {repoURL: 'oci://registry.example.com/chart'});
        const testRenderer = renderer.create(<ApplicationIcon app={app} />);
        const testInstance = testRenderer.root;
        const imgs = testInstance.findAllByType('img');
        expect(imgs.length).toBe(0);
        const icons = testInstance.findAllByType('i');
        expect(icons.length).toBe(1);
        expect(icons[0].props.className).toBe('icon argo-icon-oci');
    });

    it('should respect custom size prop', () => {
        const app = makeApp({'argocd.argoproj.io/icon': 'https://example.com/icon.png'});
        const testRenderer = renderer.create(<ApplicationIcon app={app} size='large' />);
        const testInstance = testRenderer.root;
        const imgs = testInstance.findAllByType('img');
        expect(imgs.length).toBe(1);
        expect(imgs[0].props.className).toContain('application-icon--large');
    });
});
