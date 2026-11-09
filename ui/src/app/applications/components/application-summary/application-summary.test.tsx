import * as React from 'react';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {ApplicationSummaryExternalURLs} from './application-summary-external-urls';

describe('ApplicationSummaryExternalURLs', () => {
    it('trims regex paths from the displayed URL and href', () => {
        const url = 'https://preview.example.com/123(/|$)(.*)';
        const expected = 'https://preview.example.com/123';

        render(<ApplicationSummaryExternalURLs urls={[url]} />);

        const link = screen.getByRole('link', {name: expected});
        expect(link).toHaveTextContent(expected);
        expect(link).toHaveAttribute('href', expected);
    });

    it('continues to support titled external URLs', () => {
        const url = 'https://preview.example.com/123(/|$)(.*)';

        render(<ApplicationSummaryExternalURLs urls={['Preview|' + url]} />);

        const link = screen.getByRole('link', {name: 'Preview'});
        expect(link).toHaveTextContent('Preview');
        expect(link).toHaveAttribute('href', 'https://preview.example.com/123');
    });
});
