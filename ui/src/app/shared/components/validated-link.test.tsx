import {render, screen} from '@testing-library/react';
import * as React from 'react';
import {ValidatedLink} from './validated-link';

describe('ValidatedLink', () => {
    test.each(['javascript:alert(1)', 'JaVaScRiPt:alert(1)', 'data:text/html,<script>alert(1)</script>', 'vbscript:msgbox(1)'])('does not render unsafe URL %s', url => {
        render(<ValidatedLink url={url}>external link</ValidatedLink>);

        expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });

    test('renders a valid HTTPS link with the supplied security attributes', () => {
        render(
            <ValidatedLink url='https://example.com/chart' target='_blank' rel='noopener noreferrer'>
                chart home
            </ValidatedLink>
        );

        const link = screen.getByRole('link');
        expect(link).toHaveAttribute('href', 'https://example.com/chart');
        expect(link).toHaveAttribute('target', '_blank');
        expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });
});
