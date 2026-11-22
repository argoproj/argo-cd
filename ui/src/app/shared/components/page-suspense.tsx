import {Spinner} from './spinner';
import * as React from 'react';

// Suspends only the lazily loaded page, not the layout around it. When a Suspense boundary above the
// layout shows its fallback, React hides the layout and its DataLoader unsubscribes from the view
// preferences without subscribing again, so the sidebar and layout stop following preference changes.
export const PageSuspense = ({children}: {children: React.ReactNode}) => (
    <React.Suspense
        fallback={
            <div style={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh'}}>
                <Spinner show={true} />
            </div>
        }>
        {children}
    </React.Suspense>
);
