import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {ResourceIcon} from './resource-icon';

// Mock the resourceIcons and resourceCustomizations
jest.mock('./resources', () => ({
    resourceIcons: new Map([
        ['Ingress', 'ing'],
        ['ConfigMap', 'cm'],
        ['Deployment', 'deploy'],
        ['Service', 'svc']
    ])
}));

jest.mock('./resource-customizations', () => ({
    resourceIconGroups: {
        '*.crossplane.io': true,
        '*.fluxcd.io': true,
        'cert-manager.io': true
    }
}));

describe('ResourceIcon', () => {
    const renderResourceIcon = (group: string, kind: string) => {
        let testRenderer: renderer.ReactTestRenderer;
        renderer.act(() => {
            testRenderer = renderer.create(<ResourceIcon group={group} kind={kind} />);
        });
        return testRenderer;
    };

    describe('kind-based icons (no group)', () => {
        it('should show kind-based icon for ConfigMap without group', () => {
            const testRenderer = renderResourceIcon('', 'ConfigMap');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0].props.src).toBe('assets/images/resources/cm.svg');
        });

        it('should show kind-based icon for Deployment without group', () => {
            const testRenderer = renderResourceIcon('', 'Deployment');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0].props.src).toBe('assets/images/resources/deploy.svg');
        });
    });

    describe('group-based icons (with matching group)', () => {
        it('should show group-based icon for exact group match', () => {
            const testRenderer = renderResourceIcon('cert-manager.io', 'Certificate');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0].props.src).toBe('assets/images/resources/cert-manager.io/icon.svg');
        });

        it('should show group-based icon for wildcard group match (crossplane)', () => {
            const testRenderer = renderResourceIcon('pkg.crossplane.io', 'Provider');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            // Wildcard '*' should be replaced with '_' in the path
            expect(imgs[0].props.src).toBe('assets/images/resources/_.crossplane.io/icon.svg');

            const complexTestRenderer = renderResourceIcon('identify.provider.crossplane.io', 'Provider');
            const complexTestInstance = complexTestRenderer.root;
            const complexImgs = complexTestInstance.findAllByType('img');
            expect(complexImgs.length).toBeGreaterThan(0);
            // Wildcard '*' should be replaced with '_' in the path
            expect(complexImgs[0].props.src).toBe('assets/images/resources/_.crossplane.io/icon.svg');
        });

        it('should show group-based icon for wildcard group match (fluxcd)', () => {
            const testRenderer = renderResourceIcon('source.fluxcd.io', 'GitRepository');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0].props.src).toBe('assets/images/resources/_.fluxcd.io/icon.svg');
        });
    });

    describe('fallback to kind-based icons (with non-matching group) - THIS IS THE BUG FIX', () => {
        it('should fallback to kind-based icon for Ingress with networking.k8s.io group', () => {
            // This is the main bug fix test case
            // Ingress has group 'networking.k8s.io' which is NOT in resourceCustomizations
            // But Ingress IS in resourceIcons, so it should still show the icon
            const testRenderer = renderResourceIcon('networking.k8s.io', 'Ingress');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0].props.src).toBe('assets/images/resources/ing.svg');
        });

        it('should fallback to kind-based icon for Service with core group', () => {
            const testRenderer = renderResourceIcon('', 'Service');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0].props.src).toBe('assets/images/resources/svc.svg');
        });
    });

    describe('fallback to initials (no matching group or kind)', () => {
        it('should show initials for unknown resource with unknown group', () => {
            const testRenderer = renderResourceIcon('unknown.example.io', 'UnknownResource');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBe(0);
            // Should show initials "UR" (uppercase letters from UnknownResource)
            const spans = testInstance.findAllByType('span');
            const textSpan = spans.find(s => s.children.includes('UR'));
            expect(textSpan).toBeTruthy();
        });

        it('should show initials for MyCustomKind', () => {
            const testRenderer = renderResourceIcon('', 'MyCustomKind');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBe(0);
            // Should show initials "MCK"
            const spans = testInstance.findAllByType('span');
            const textSpan = spans.find(s => s.children.includes('MCK'));
            expect(textSpan).toBeTruthy();
        });
    });

    describe('special cases', () => {
        it('should show node icon for kind=node', () => {
            const testRenderer = renderResourceIcon('', 'node');
            const testInstance = testRenderer.root;
            const imgs = testInstance.findAllByType('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0].props.src).toBe('assets/images/infrastructure_components/node.svg');
        });

        it('should show application icon for kind=Application', () => {
            const testRenderer = renderResourceIcon('', 'Application');
            const testInstance = testRenderer.root;
            const icons = testInstance.findAll(node => node.type === 'i' && typeof node.props.className === 'string' && node.props.className.includes('argo-icon-application'));
            expect(icons.length).toBeGreaterThan(0);
        });
    });
});
