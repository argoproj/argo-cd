import * as React from 'react';
import {Tooltip} from 'argo-ui';

export type ContainerGroup = {offset: number; containers: {name: string}[]};

// ContainerSelector is a component that renders a dropdown menu of containers
export const ContainerSelector = ({
    containerGroups,
    containerName,
    onClickContainer
}: {
    containerGroups?: ContainerGroup[];
    containerName: string;
    onClickContainer: (group: ContainerGroup, index: number, logs: string) => void;
}) => {
    if (!containerGroups) {
        return <></>;
    }

    const containers = containerGroups?.reduce((acc, group) => acc.concat(group.containers), []);
    const containerNames = containers?.map(container => container.name);
    const containerGroup = (n: string) => {
        return containerGroups?.find(group => group.containers?.find(container => container.name === n));
    };
    const containerIndex = (n: string) => {
        return containerGroup(n)?.containers.findIndex(container => container.name === n);
    };
    if (containerNames.length <= 1) return <></>;
    return (
        <Tooltip content='Select a container to view logs' interactive={false}>
            <select className='argo-field' value={containerName} onChange={e => onClickContainer(containerGroup(e.target.value), containerIndex(e.target.value), 'logs')}>
                {containerNames.map(n => (
                    <option key={n} value={n}>
                        {n}
                    </option>
                ))}
            </select>
        </Tooltip>
    );
};
