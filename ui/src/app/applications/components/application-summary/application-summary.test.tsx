import * as React from 'react';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {ApplicationSummaryExternalURLs} from './application-summary';

describe('ApplicationSummaryExternalURLs', () => {
    it('preserves regex URLs containing pipe characters in the displayed URL and href', () => {
        const url = 'https://preview.example.com/123(/|$)(.*)';

        render(<ApplicationSummaryExternalURLs urls={[url]} />);

        const link = screen.getByRole('link', {name: url});
        expect(link).toHaveTextContent(url);
        expect(link).toHaveAttribute('href', url);
    });

    it('continues to support titled external URLs', () => {
        const url = 'https://preview.example.com/app';

        render(<ApplicationSummaryExternalURLs urls={['Preview|' + url]} />);

        const link = screen.getByRole('link', {name: 'Preview'});
        expect(link).toHaveTextContent('Preview');
        expect(link).toHaveAttribute('href', url);
    });
});
