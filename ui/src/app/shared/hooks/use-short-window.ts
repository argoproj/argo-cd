import * as React from 'react';

/** Viewport heights at or below this are treated as a short window for compacting the application header. */
export const SHORT_WINDOW_MAX_HEIGHT_PX = 700;

export const SHORT_WINDOW_MEDIA_QUERY = `(max-height: ${SHORT_WINDOW_MAX_HEIGHT_PX}px)`;

/** Whether the current window's viewport is short. This is per-window and is not persisted. */
export function useShortWindow(): boolean {
    const [isShort, setIsShort] = React.useState(() => window.matchMedia(SHORT_WINDOW_MEDIA_QUERY).matches);

    React.useEffect(() => {
        const mediaQuery = window.matchMedia(SHORT_WINDOW_MEDIA_QUERY);
        const onChange = () => setIsShort(mediaQuery.matches);
        onChange();
        mediaQuery.addEventListener('change', onChange);
        return () => mediaQuery.removeEventListener('change', onChange);
    }, []);

    return isShort;
}
