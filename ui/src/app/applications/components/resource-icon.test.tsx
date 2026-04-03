import * as React from 'react';
import {render, screen} from '@testing-library/react';
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
        'cert-manager.io': true,
        'promoter.argoproj.io': true
    }
}));

describe('ResourceIcon', () => {
    const renderResourceIcon = (group: string, kind: string) => {
        render(<ResourceIcon group={group} kind={kind} />);
    };

    describe('kind-based icons (no group)', () => {
        it('should show kind-based icon for ConfigMap without group', () => {
            renderResourceIcon('', 'ConfigMap');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/cm.svg');
        });

        it('should show kind-based icon for Deployment without group', () => {
            renderResourceIcon('', 'Deployment');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/deploy.svg');
        });
    });

    describe('group-based icons (with matching group)', () => {
        it('should show group-based icon for exact group match', () => {
            renderResourceIcon('cert-manager.io', 'Certificate');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/cert-manager.io/icon.svg');
        });

        it('should show group-based icon for wildcard group match (crossplane)', () => {
            renderResourceIcon('pkg.crossplane.io', 'Provider');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            // Wildcard '*' should be replaced with '_' in the path
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/_.crossplane.io/icon.svg');

            const {getAllByRole} = render(<ResourceIcon group='identify.provider.crossplane.io' kind='Provider' />);
            const complexImgs = getAllByRole('img');
            expect(complexImgs.length).toBeGreaterThan(0);
            // Wildcard '*' should be replaced with '_' in the path
            expect(complexImgs[0]).toHaveAttribute('src', 'assets/images/resources/_.crossplane.io/icon.svg');
        });

        it('should show group-based icon for wildcard group match (fluxcd)', () => {
            renderResourceIcon('source.fluxcd.io', 'GitRepository');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/_.fluxcd.io/icon.svg');
        });

        it('should show group-based icon for promoter.argoproj.io', () => {
            render(<ResourceIcon group='promoter.argoproj.io' kind='PromotionStrategy' />);
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/promoter.argoproj.io/icon.svg');
        });
    });

    describe('fallback to kind-based icons (with non-matching group) - THIS IS THE BUG FIX', () => {
        it('should fallback to kind-based icon for Ingress with networking.k8s.io group', () => {
            // This is the main bug fix test case
            // Ingress has group 'networking.k8s.io' which is NOT in resourceCustomizations
            // But Ingress IS in resourceIcons, so it should still show the icon
            renderResourceIcon('networking.k8s.io', 'Ingress');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/ing.svg');
        });

        it('should fallback to kind-based icon for Service with core group', () => {
            renderResourceIcon('', 'Service');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/resources/svc.svg');
        });
    });

    describe('fallback to initials (no matching group or kind)', () => {
        it('should show initials for unknown resource with unknown group', () => {
            renderResourceIcon('unknown.example.io', 'UnknownResource');
            const imgs = screen.queryAllByRole('img');
            expect(imgs.length).toBe(0);
            // Should show initials "UR" (uppercase letters from UnknownResource)
            expect(screen.getByText('UR')).toBeInTheDocument();
        });

        it('should show initials for MyCustomKind', () => {
            renderResourceIcon('', 'MyCustomKind');
            const imgs = screen.queryAllByRole('img');
            expect(imgs.length).toBe(0);
            // Should show initials "MCK"
            expect(screen.getByText('MCK')).toBeInTheDocument();
        });
    });

    describe('special cases', () => {
        it('should show node icon for kind=node', () => {
            renderResourceIcon('', 'node');
            const imgs = screen.getAllByRole('img');
            expect(imgs.length).toBeGreaterThan(0);
            expect(imgs[0]).toHaveAttribute('src', 'assets/images/infrastructure_components/node.svg');
        });

        it('should show application icon for kind=Application', () => {
            renderResourceIcon('', 'Application');
            const icon = document.querySelector('i.argo-icon-application');
            expect(icon).toBeTruthy();
        });
    });
});
