import * as React from 'react';

export enum Key {
    ENTER = 13,
    LEFT = 37,
    UP = 38,
    RIGHT = 39,
    DOWN = 40,
    SLASH = 191
}

export const useKeyPress = (key: Key | Key[], action: () => void) => {
    React.useEffect(() => {
        const handlePress = (e: KeyboardEvent) => {
            const keys = Array.isArray(key) ? key : [key];
            for (const k of keys) {
                if (e.keyCode === k) {
                    action();
                    e.preventDefault();
                }
            }
        };
        document.addEventListener('keydown', handlePress);
        return () => {
            document.removeEventListener('keydown', handlePress);
        };
    });
};
