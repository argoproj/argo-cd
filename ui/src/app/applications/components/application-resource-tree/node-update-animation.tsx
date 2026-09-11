import * as React from 'react';

/**
 * When the resource version changes, we want to trigger an animation to indicate that the resource has been updated. This component will be rendered as a child of the node and will update itself when the resource version changes leading to a re-render, which triggers the animation.
 * @param props Resource version
 * @returns
 */
export const NodeUpdateAnimation = (props: {resourceVersion: string}) => {
    const [animate, setAnimation] = React.useState(false);
    React.useEffect(() => {
        return () => {
            setAnimation(true);
        };
    }, [props.resourceVersion]);

    return animate && <div key={props.resourceVersion} className='application-resource-tree__node-animation' />;
};
