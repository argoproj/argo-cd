import * as React from 'react';
import {useState, useRef, useEffect} from 'react';

interface Props {
    resourceVersion: string;
}

export function NodeUpdateAnimation({resourceVersion}: Props) {
    const [ready, setReady] = useState(false);
    const prevVersionRef = useRef<string | null>(null);

    useEffect(() => {
        if (prevVersionRef.current !== null && prevVersionRef.current !== resourceVersion) {
            setReady(true);
        }
        prevVersionRef.current = resourceVersion;
    }, [resourceVersion]);

    return ready ? <div key={resourceVersion} className='application-resource-tree__node-animation' /> : null;
}
