import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {RevisionMetadataPanel} from './revision-metadata-panel';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            revisionMetadata: jest.fn(() =>
                Promise.resolve({
                    author: 'Test Author',
                    date: '2026-01-01T00:00:00Z',
                    message: 'Test commit message',
                    tags: ['v1.0.0'],
                    signatureInfo: ''
                })
            ),
            ociMetadata: jest.fn(() =>
                Promise.resolve({
                    authors: 'OCI Author',
                    createdAt: '2026-01-01T00:00:00Z',
                    description: 'Test OCI description',
                    version: '1.0.0'
                })
            )
        }
    }
}));

describe('RevisionMetadataPanel', () => {
    const defaultProps = {
        appName: 'test-app',
        appNamespace: 'default',
        revision: 'abc123',
        versionId: 1
    };

    it('returns null for helm type', () => {
        const component = renderer.create(<RevisionMetadataPanel {...defaultProps} type='helm' />);
        expect(component.toJSON()).toBeNull();
    });

    it('does NOT return null for oci type', () => {
        const component = renderer.create(<RevisionMetadataPanel {...defaultProps} type='oci' />);
        // The component should render (not null) - this is the bug fix
        expect(component.toJSON()).not.toBeNull();
    });

    it('does NOT return null for git type', () => {
        const component = renderer.create(<RevisionMetadataPanel {...defaultProps} type='git' />);
        expect(component.toJSON()).not.toBeNull();
    });

    it('calls ociMetadata service for oci type', () => {
        const {services} = require('../../../shared/services');
        renderer.create(<RevisionMetadataPanel {...defaultProps} type='oci' />);
        expect(services.applications.ociMetadata).toHaveBeenCalledWith('test-app', 'default', 'abc123', 0, 1);
    });

    it('calls revisionMetadata service for git type', () => {
        const {services} = require('../../../shared/services');
        renderer.create(<RevisionMetadataPanel {...defaultProps} type='git' />);
        expect(services.applications.revisionMetadata).toHaveBeenCalledWith('test-app', 'default', 'abc123', 0, 1);
    });
});
