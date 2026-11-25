import * as React from 'react';

const VISIBILITY_DEBOUNCE = 500;

/** Whether the browser tab is visible, settling only once the user stops switching tabs back and forth. */
export function usePageVisible(): boolean {
    const [visible, setVisible] = React.useState(() => !document.hidden);
    React.useEffect(() => {
        let timer: ReturnType<typeof setTimeout>;
        const onChange = () => {
            clearTimeout(timer);
            timer = setTimeout(() => setVisible(!document.hidden), VISIBILITY_DEBOUNCE);
        };
        document.addEventListener('visibilitychange', onChange);
        return () => {
            clearTimeout(timer);
            document.removeEventListener('visibilitychange', onChange);
        };
    }, []);
    return visible;
}
