import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {ValidatedLink} from './validated-link';

describe('ValidatedLink', () => {
    test.each(['javascript:alert(1)', 'JaVaScRiPt:alert(1)', 'data:text/html,<script>alert(1)</script>', 'vbscript:msgbox(1)'])('does not render unsafe URL %s', url => {
        const tree = renderer.create(<ValidatedLink url={url}>external link</ValidatedLink>);

        expect(tree.root.findAllByType('a')).toHaveLength(0);
    });

    test('renders a valid HTTPS link with the supplied security attributes', () => {
        const tree = renderer.create(
            <ValidatedLink url='https://example.com/chart' target='_blank' rel='noopener noreferrer'>
                chart home
            </ValidatedLink>
        );

        const link = tree.root.findByType('a');
        expect(link.props.href).toBe('https://example.com/chart');
        expect(link.props.target).toBe('_blank');
        expect(link.props.rel).toBe('noopener noreferrer');
    });
});
