import * as React from 'react';

export const InitiatedBy = (props: {username: string; automated: boolean}) => {
    const initiator = props.automated ? 'automated sync policy' : props.username || 'Unknown';
    return <span>{initiator}</span>;
};
